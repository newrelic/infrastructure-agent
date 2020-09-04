// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/trace"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const (
	DATA_DIR_MODE               = 0755 // default mode for data directories
	DATA_FILE_MODE              = 0644 // default mode for data files
	CACHE_DIR                   = ".delta_repo"
	SAMPLING_REPO               = ".sampling_repo"
	CACHE_ID_FILE               = "delta_id_cache.json"
	UNSENT_DELTA_JOURNAL_EXT    = ".pending"
	ARCHIVE_DELTA_JOURNAL_EXT   = ".sent"
	NO_DELTA_ID                 = 0
	localEntityFolder           = "__nria_localentity"
	DisableInventorySplit       = 0
	lastSuccessSubmissionFolder = "last_success"
	lastEntityIDFolder          = "last_entityID"
)

var EMPTY_DELTA = []byte{'{', '}'}
var NULL = []byte{'n', 'u', 'l', 'l'}
var ErrNoPreviousSuccessSubmissionTime = fmt.Errorf("no previous success submission time")

var slog = log.WithComponent("Delta Store")

// Folders that do not belong to entities nor plugins, so they have to be ignored
var nonEntityFolders = map[string]bool{
	CACHE_DIR:     true,
	SAMPLING_REPO: true,
}

type delta struct {
	value []byte
	full  bool
}

// Performs an in-place removal of any nil map values within the given object
func removeNils(data interface{}) {
	switch dt := data.(type) {
	case map[string]interface{}:
		for k, v := range dt {
			if v == nil {
				delete(dt, k)
			} else {
				removeNils(v)
			}
		}
	case []interface{}:
		for _, v := range dt {
			removeNils(v)
		}
	default:
		return
	}
}

type Storage interface {
	ReadDeltas(entityKey string) ([]inventoryapi.RawDeltaBlock, error)
	RemoveEntity(entityKey string) error
	CompactStorage(entityKey string, threshold uint64) (err error)
	ResetAllDeltas(entityKey string)
	UpdateState(entityKey string, deltas []*inventoryapi.RawDelta, deltaStateResults *inventoryapi.DeltaStateMap)
	SaveState() (err error)
}

// Store handles information about the storage of Deltas.
type Store struct {
	// DataDir holds the agent data directory
	DataDir string
	// CacheDir holds the agent cache directory
	CacheDir string
	// if <= 0, the inventory splitting is disabled
	maxInventorySize int
	// defaultEntityKey holds the agent entity name
	defaultEntityKey string
	// plugins stores the information about the available plugins
	plugins pluginSource2Info
	// stores time of last success submission of inventory to backend
	lastSuccessSubmission time.Time
}

// NewStore creates a new Store and returns a pointer to it. If maxInventorySize <= 0, the inventory splitting is disabled
func NewStore(dataDir string, defaultEntityKey string, maxInventorySize int) *Store {
	if defaultEntityKey == "" {
		slog.Error("creating delta store: default entity ID can't be empty")
		panic("default entity ID can't be empty")
	}

	d := &Store{
		DataDir:          dataDir,
		CacheDir:         filepath.Join(dataDir, CACHE_DIR),
		maxInventorySize: maxInventorySize,
		defaultEntityKey: defaultEntityKey,
		plugins:          make(pluginSource2Info),
	}

	// Nice2Have: remove side effects from constructor
	if err := d.createDataStore(); err != nil {
		slog.WithError(err).Error("can't initialize data store")
		panic(err)
	}

	// Nice2Have: remove side effects from constructor
	cachedDeltaPath := filepath.Join(d.CacheDir, CACHE_ID_FILE)
	if err := d.readPluginIDMap(cachedDeltaPath); err != nil {
		slog.WithError(err).WithField("file", cachedDeltaPath).Error("can't initialize plugin-id map")
		err = os.Remove(cachedDeltaPath)
		if err != nil {
			panic(err)
		}
	}

	return d
}

func (s *Store) createDataStore() (err error) {
	if err = disk.MkdirAll(s.DataDir, DATA_DIR_MODE); err != nil {
		return fmt.Errorf("can't create data directory: %s err: %s", s.DataDir, err)
	}

	if _, err = os.Stat(s.CacheDir); err == nil {
		return
	}

	if err = disk.MkdirAll(s.CacheDir, DATA_DIR_MODE); err != nil {
		return fmt.Errorf("can't create cache directory: %s, err: %s", s.CacheDir, err)
	}

	samplingDir := filepath.Join(s.DataDir, SAMPLING_REPO)
	if err = disk.MkdirAll(samplingDir, DATA_DIR_MODE); err != nil {
		return fmt.Errorf("can't create data sampling directory: %s, err: %s", samplingDir, err)
	}

	return
}

func (s *Store) archiveFilePath(pluginItem *PluginInfo, entityKey string) string {
	file := s.cachedFilePath(pluginItem, entityKey)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(file, filepath.Ext(file)), ARCHIVE_DELTA_JOURNAL_EXT)
}

func (s *Store) DeltaFilePath(pluginItem *PluginInfo, entityKey string) string {
	file := s.cachedFilePath(pluginItem, entityKey)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(file, filepath.Ext(file)), UNSENT_DELTA_JOURNAL_EXT)
}

func (s *Store) cachedDirPath(pluginItem *PluginInfo, entityKey string) string {
	return filepath.Join(s.CacheDir,
		pluginItem.Plugin,
		s.entityFolder(entityKey))
}

func (s *Store) cachedFilePath(pluginItem *PluginInfo, entityKey string) string {
	return filepath.Join(s.CacheDir,
		pluginItem.Plugin,
		s.entityFolder(entityKey),
		pluginItem.FileName)
}

func (s *Store) SourceFilePath(pluginItem *PluginInfo, entityKey string) string {
	return filepath.Join(s.DataDir,
		pluginItem.Plugin,
		s.entityFolder(entityKey),
		pluginItem.FileName)
}

func (s *Store) PluginDirPath(pluginCategory, entityKey string) string {
	return filepath.Join(s.DataDir, pluginCategory, s.entityFolder(entityKey))
}

func (s *Store) clearPluginDeltaStore(pluginItem *PluginInfo, entityKey string) (err error) {
	// Clear the cachedFile and deltas
	cachedFilePath := s.cachedFilePath(pluginItem, entityKey)
	deltaFilePath := s.DeltaFilePath(pluginItem, entityKey)
	archiveFilePath := s.archiveFilePath(pluginItem, entityKey)
	helpers.DebugStackf("Clearing delta store for plugin %s and entity %s: %s, %s, %s",
		pluginItem.Source, entityKey, cachedFilePath, deltaFilePath, archiveFilePath)
	_ = os.Remove(cachedFilePath)
	_ = os.Remove(deltaFilePath)
	_ = os.Remove(archiveFilePath)
	return
}

func (s *Store) compactCacheStorage(entityKey string, threshold uint64) (err error) {
	// Strategy:
	// For any plugins that don't exist anymore, we can complete clean those out
	// For plugins that do exist with N generations of data, remove all sent generations

	if activePlugins, err := s.collectPluginFiles(s.DataDir, entityKey, helpers.JsonFilesRegexp); err == nil {
		if reapedPlugins, err := s.collectPluginFiles(s.CacheDir, entityKey, helpers.JsonFilesRegexp); err == nil {
			// clear out unused plugins
			removedPlugins := make(map[string]*PluginInfo)
			for _, plugin := range reapedPlugins {
				removedPlugins[plugin.Source] = plugin
			}
			for _, plugin := range activePlugins {
				delete(removedPlugins, plugin.Source)
			}
			for _, p := range removedPlugins {
				_ = s.clearPluginDeltaStore(p, entityKey)
				delete(s.plugins, p.Source)
			}

			// Now for the active ones, remove their archives
			for _, p := range activePlugins {
				archiveFilePath := s.archiveFilePath(p, entityKey)
				_ = os.Remove(archiveFilePath)
			}

		}
	}
	return
}

// CompactStorage reduces the size of the Delta Storage
func (s *Store) CompactStorage(entityKey string, threshold uint64) (err error) {
	var repoSize, newRepoSize uint64
	repoSize, err = s.StorageSize(s.CacheDir)
	if err == nil && repoSize > 0 && repoSize > threshold {
		cslog := slog.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{"repoSize": repoSize, "threshold": threshold, "entityKey": entityKey}
		})

		cslog.Debug("Local repo size exceeds compaction threshold. Compacting.")
		if err = s.compactCacheStorage(entityKey, threshold); err != nil {
			return
		}
		newRepoSize, err = s.StorageSize(s.CacheDir)
		if nil != err {
			return
		}

		cslog.WithFieldsF(func() logrus.Fields {
			savedPct := (float64(repoSize-newRepoSize) / float64(repoSize)) * 100
			return logrus.Fields{"newRepoSize": newRepoSize, "savedPercentage": savedPct}
		}).Debug("Local repo compacted.")

		err = s.SaveState()
	}
	return
}

// StorageSize returns the size used in bytes of all the loose objects in the cache, non-inclusive of dirs
func (s *Store) StorageSize(path string) (uint64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return uint64(size), err
}

func (s *Store) archivePlugin(pluginItem *PluginInfo, entityKey string) (err error) {
	var buf []byte
	buf, err = s.readIndividualPluginDeltas(pluginItem, entityKey)
	if err != nil {
		return
	}

	buf = s.wrapBuffer(buf, '[', ']', ",")
	var deltas []*inventoryapi.RawDelta
	if err = json.Unmarshal(buf, &deltas); err != nil {
		slog.WithFields(logrus.Fields{"plugin": pluginItem.ID(), "entityKey": entityKey}).
			WithError(err).Error("archivePlugin can't unmarshal raw deltas?")
		return
	}

	keepDeltas := make([]*inventoryapi.RawDelta, 0)
	archiveDeltas := make([]*inventoryapi.RawDelta, 0)
	_, ok := s.plugins[pluginItem.Source]
	for _, d := range deltas {
		if ok && d.ID <= pluginItem.lastSentID(entityKey) {
			// backend already has this data
			archiveDeltas = append(archiveDeltas, d)
		} else {
			keepDeltas = append(keepDeltas, d)
		}
	}

	err = s.rewriteDeltas(s.archiveFilePath(pluginItem, entityKey), os.O_CREATE|os.O_APPEND|os.O_WRONLY, archiveDeltas)
	if err != nil {
		return
	}

	return s.rewriteDeltas(s.DeltaFilePath(pluginItem, entityKey), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, keepDeltas)
}

// ResetAllDeltas clears the plugin delta store for all the existing plugins
func (s *Store) ResetAllDeltas(entityKey string) {
	if s.plugins != nil {
		for _, plugin := range s.plugins {
			_ = s.clearPluginDeltaStore(plugin, entityKey)
		}
	}
}

// UpdateState updates in disk the state of the deltas according to the passed PostDeltaBody, whose their ExternalKeys
// field may be empty.
func (s *Store) UpdateState(entityKey string, deltas []*inventoryapi.RawDelta, deltaStateResults *inventoryapi.DeltaStateMap) {
	sentPlugins := make([]string, len(deltas))

	// record what was sent and archive
	for _, d := range deltas {
		var dResult *inventoryapi.DeltaState
		if deltaStateResults != nil {
			dResult, _ = (*deltaStateResults)[d.Source]
		}
		s.updateLastDeltaSent(entityKey, d, dResult)
		sentPlugins = append(sentPlugins, d.Source)
	}

	// Clean up delta files in bulk for each plugin
	for _, source := range sentPlugins {
		plugin := s.plugins[source]
		if plugin != nil {
			ierr := s.archivePlugin(plugin, entityKey)
			if ierr != nil {
				slog.WithFields(logrus.Fields{"source": source, "entity": entityKey}).
					Debug("UpdateState: Plugin delta does not exist.")
			}
		}
	}
	return
}

func (s *Store) updateLastDeltaSent(entityKey string, dRaw *inventoryapi.RawDelta, resultHint *inventoryapi.DeltaState) {
	if s.plugins == nil {
		return
	}

	source := dRaw.Source
	id := dRaw.ID
	p, ok := s.plugins[source]
	if !ok {
		return
	}

	dslog := slog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{"entityKey": entityKey, "source": source}
	})
	if resultHint != nil {
		if resultHint.Error != nil {
			dslog.WithField("error", *resultHint.Error).
				Debug("Plugin delta submission returned a hint with an error.")
		} else {
			dslog.WithFields(logrus.Fields{
				"needsReset":   resultHint.NeedsReset,
				"lastStoredID": resultHint.LastStoredID,
				"sendNextID":   resultHint.SendNextID,
			}).Debug("Plugin delta submission returned a hint.")
		}
	} else {
		dslog.Debug("Plugin delta submission did not return any hint.")
	}

	// If we have a result Hint, we'll use that, otherwise, use the supplied id
	if resultHint != nil {
		switch {
		case resultHint.NeedsReset:
			// Fixes the situation where agent sent N but backend expected N+1. In this situation,
			// when backend sends back N+1 as the SendNextID, the agent could not tell if its delta
			// was problematic.
			s.reconciliateWithBackend(p, entityKey, resultHint)

		case resultHint.SendNextID == id+1:
			// Normal case.
			p.setLastSentID(entityKey, id)

		case resultHint.SendNextID == 0:
			// Send full. Leave delta ID values as is.
			_ = s.clearPluginDeltaStore(p, entityKey)

		case resultHint.SendNextID != id:
			// If not present, send current full Reset delta ids to use SendNextID for the numbering
			// of the next delta ids so we can fill in the gaps in the correct sequence.
			s.reconciliateWithBackend(p, entityKey, resultHint)

		case resultHint.SendNextID == id:
			// Send again? This is a no-op, set last sent id to one previous.
			dslog.WithFields(logrus.Fields{"sendNextID": id, "plugin": p}).
				Debug("Requesting to update last delta sent to identical value.")
			p.setLastSentID(entityKey, id-1)
		}
	} else if id > p.lastSentID(entityKey) {
		p.setLastSentID(entityKey, id)
	}

	dslog.WithField("plugin", source).Debug("Updating deltas.")
}

func (s *Store) reconciliateWithBackend(pi *PluginInfo, entityKey string, resultHint *inventoryapi.DeltaState) {
	_ = s.clearPluginDeltaStore(pi, entityKey)
	pi.setLastSentID(entityKey, resultHint.SendNextID-1)
	pi.setDeltaID(entityKey, resultHint.LastStoredID)
}

// SaveState writes on disk the plugin ID maps
func (s *Store) SaveState() (err error) {
	if err = s.writePluginIDMap(); err != nil {
		slog.WithError(err).Error("can't write plugin id maps")
	}
	return
}

func (s *Store) readPluginIDMap(cachedDeltaPath string) (err error) {
	ok := exists(cachedDeltaPath)
	if !ok {
		return nil
	}

	if deltaIDBytes, err := ioutil.ReadFile(cachedDeltaPath); err == nil {
		return s.loadPluginIDMap(deltaIDBytes)
	}

	return err
}

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		slog.Debugf("The provided path: %s return an error: %s", path, err)
		return false
	}
	return true
}

func (s *Store) loadPluginIDMap(deltaIDBytes []byte) (err error) {
	if len(deltaIDBytes) > 0 {
		return json.Unmarshal(deltaIDBytes, &s.plugins)
	}

	slog.Debug("Empty Plugin ID Map cache file, starting fresh.")
	return nil
}

func (s *Store) writePluginIDMap() (err error) {
	if _, err = os.Stat(s.CacheDir); err != nil {
		return
	}

	var buf []byte
	if buf, err = json.Marshal(s.plugins); err != nil {
		slog.WithError(err).Error("can't marshal id map?")
	} else {
		cachedDeltaPath := filepath.Join(s.CacheDir, CACHE_ID_FILE)
		if err = disk.WriteFile(cachedDeltaPath, buf, DATA_FILE_MODE); err != nil {
			slog.WithError(err).WithField("path", cachedDeltaPath).Error("unable to write delta cache")
		}
	}
	return
}

func (s *Store) collectPluginFiles(dir string, entityKey string, fileFilterRE *regexp.Regexp) (pluginList []*PluginInfo, err error) {
	pluginsFileInfo, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	pluginList = make([]*PluginInfo, 0, len(pluginsFileInfo))
	entityFolder := s.entityFolder(entityKey)

	for _, dirInfo := range pluginsFileInfo {
		if dirInfo != nil && dirInfo.IsDir() && !nonEntityFolders[dirInfo.Name()] {
			// Look inside each "plugin" directory to find the plugin's data files
			filesInfo, err := ioutil.ReadDir(filepath.Join(dir, dirInfo.Name(), entityFolder))
			if err != nil {
				// There is no such entity for the given plugin, so continuing
				continue
			}

			for _, fInfo := range filesInfo {
				if fInfo != nil && !fInfo.IsDir() && (fileFilterRE == nil || fileFilterRE.MatchString(fInfo.Name())) {
					pluginList = append(pluginList, newPluginInfo(dirInfo.Name(), fInfo.Name()))
				}
			}
		}
	}

	return
}

func removeNilsFromMarshaledJSON(buf []byte) (cleanBuf []byte, err error) {
	var delta interface{}
	if err = json.Unmarshal(buf, &delta); err != nil {
		slog.WithError(err).Error("can't unmarshal and remove nils")
		return
	}

	removeNils(delta)
	cleanBuf, err = json.Marshal(delta)
	if err != nil {
		slog.WithError(err).Error("can't marshal de-nil'd delta")
		return
	}

	return
}

func (s *Store) getDeltaFromJSON(previous, current []byte) (delta []byte, err error) {
	// A simple bytes comparison to prevent UnMarshalling
	if bytes.Equal(previous, current) {
		delta = EMPTY_DELTA
		return
	}

	return jsonpatch.CreateMergePatch(previous, current)
}

func (s *Store) rewriteDeltas(deltaFilePath string, flag int, deltas []*inventoryapi.RawDelta) (err error) {
	f, err := disk.OpenFile(deltaFilePath, flag, DATA_FILE_MODE)
	if err != nil {
		slog.WithField("path", deltaFilePath).WithError(err).Error("can't open delta journal file")
		return
	}
	defer f.Close()

	if len(deltas) > 0 {
		var deltaBuf []byte
		if deltaBuf, err = json.Marshal(deltas); err == nil {
			// strip the square brackets, write as one blob
			deltaBuf = bytes.Trim(deltaBuf, "[]")
			err = s.writeDelta(f, deltaBuf)
		}
	}
	return
}

func (s *Store) writeDelta(f *os.File, deltaBuf []byte) (err error) {
	if _, err = f.Write(deltaBuf); err != nil {
		slog.WithError(err).Error("can't write journal entry")
		return
	}
	if _, err = f.Write([]byte{','}); err != nil {
		slog.WithError(err).Error("can't write journal entry terminator")
	}
	return
}

func (s *Store) storeDelta(pluginItem *PluginInfo, entityKey string, d delta) (err error) {
	// FS mgmt
	dir := s.cachedDirPath(pluginItem, entityKey)
	if err = disk.MkdirAll(dir, DATA_DIR_MODE); err != nil {
		slog.WithFields(logrus.Fields{
			"path":   dir,
			"plugin": pluginItem.ID(),
		}).WithError(err).Error("could not create cache directory")
		return
	}

	file := s.DeltaFilePath(pluginItem, entityKey)
	f, err := disk.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, DATA_FILE_MODE)
	if err != nil {
		slog.WithFields(logrus.Fields{
			"path":   file,
			"plugin": pluginItem.ID(),
		}).WithError(err).Error("can't open delta journal file")
		return
	}
	defer f.Close()

	// format raw diff
	var diff map[string]interface{}
	if err = json.Unmarshal(d.value, &diff); err != nil {
		return fmt.Errorf("error unmarshaling file %s: %s", file, err)
	}

	// increase ID
	if _, ok := s.plugins[pluginItem.Source]; !ok {
		s.plugins[pluginItem.Source] = pluginItem
	}
	s.plugins[pluginItem.Source].increaseDeltaID(entityKey)

	// persist raw
	dRaw := &inventoryapi.RawDelta{
		Source:    pluginItem.Source,
		ID:        s.plugins[pluginItem.Source].deltaID(entityKey),
		Timestamp: time.Now().Unix(),
		Diff:      diff,
		FullDiff:  d.full,
	}
	var deltaBuf []byte
	if deltaBuf, err = json.Marshal(dRaw); err == nil {
		err = s.writeDelta(f, deltaBuf)
	}

	return
}

func (s *Store) wrapBuffer(buf []byte, openWrapChar, closeWrapChar byte, strip string) (wrapped []byte) {
	if len(strip) > 0 {
		buf = bytes.TrimRight(buf, strip)
	}
	wrapped = []byte{openWrapChar}
	wrapped = append(wrapped, buf...)
	return append(wrapped, closeWrapChar)
}

// The deltas are a list of json hashes WITHOUT the surrounding square brackets
func (s *Store) readIndividualPluginDeltas(plugin *PluginInfo, entityKey string) (buf []byte, err error) {
	deltaFilePath := s.DeltaFilePath(plugin, entityKey)
	if _, err = os.Stat(deltaFilePath); err == nil {
		if buf, err = ioutil.ReadFile(deltaFilePath); err != nil {
			slog.WithField("path", deltaFilePath).WithError(err).Error("can't read delta file")
		}
	}
	return
}

// Returns deltas grouped in buffers of size <= maxGroupSize
func (s *Store) readAllPluginDeltas(plugins []*PluginInfo, entityKey string) ([][]byte, error) {
	allDeltas := make([][]byte, 0)
	buf := make([]byte, 0)
	bufferSize := 0
	for _, plugin := range plugins {
		diff, err := s.readIndividualPluginDeltas(plugin, entityKey)
		diffLen := len(diff)
		if err == nil && diffLen > 0 {
			if bufferSize+diffLen > s.maxInventorySize {
				allDeltas = append(allDeltas, buf)
				buf = make([]byte, 0)
				bufferSize = 0
			}
			buf = append(buf, diff...)
			bufferSize += diffLen
		}
	}
	if bufferSize > 0 {
		allDeltas = append(allDeltas, buf)
	}
	return allDeltas, nil
}

// Legacy method that implemented delta reading before the splitting mechanism was added
func (s *Store) readAllPluginDeltasWithoutSplitting(plugins []*PluginInfo, entityKey string) (buf []byte, err error) {
	buf = []byte{}
	for _, plugin := range plugins {
		diff, err := s.readIndividualPluginDeltas(plugin, entityKey)
		if err == nil && len(diff) > 0 {
			buf = append(buf, diff...)
		}
	}
	return buf, nil
}

func (s *Store) cleanPluginDeltas(plugins []*PluginInfo, entityKey string) (err error) {
	for _, plugin := range plugins {
		var delta inventoryapi.RawDelta
		if buf, err := s.readIndividualPluginDeltas(plugin, entityKey); err == nil {
			if err = json.Unmarshal(buf, &delta); err != nil {
				deltaFilePath := s.DeltaFilePath(plugin, entityKey)
				cslog := slog.WithFieldsF(func() logrus.Fields {
					return logrus.Fields{
						"entity": entityKey,
						"plugin": plugin.ID(),
						"path":   deltaFilePath,
					}
				})

				if err = disk.WriteFile(deltaFilePath, []byte(``), DATA_FILE_MODE); err != nil {
					cslog.WithError(err).Error("can't clean delta file")
					return err
				}
				cslog.Debug("Cleaned plugin file.")
			}
		}
	}
	return
}

// ReadDeltas collects the plugins and read their deltas, grouped in blocks of size < maxInventorySize
func (s *Store) ReadDeltas(entityKey string) ([]inventoryapi.RawDeltaBlock, error) {
	// Walk through all active plugins and see if each has any deltas,
	// and collect them if so
	llog := slog.WithField("entity", entityKey)
	reapedPlugins, err := s.collectPluginFiles(s.CacheDir, entityKey, helpers.JsonFilesRegexp)
	if err != nil {
		llog.WithError(err).WithField("path", s.CacheDir).Error("can't get plugins in cache directory")
		return nil, err
	}

	var buffers [][]byte
	if s.maxInventorySize <= DisableInventorySplit {
		buffer, err := s.readAllPluginDeltasWithoutSplitting(reapedPlugins, entityKey)
		if err != nil {
			return nil, err
		}
		buffers = [][]byte{buffer}
	} else {
		buffers, err = s.readAllPluginDeltas(reapedPlugins, entityKey)
		if err != nil {
			return nil, err
		}
	}

	deltas := make([]inventoryapi.RawDeltaBlock, 0)
	for _, buf := range buffers {
		deltasGroup := make([]*inventoryapi.RawDelta, 0)
		buf = s.wrapBuffer(buf, '[', ']', ",")
		if err = json.Unmarshal(buf, &deltasGroup); err != nil {
			llog.WithError(err).Error("ReadDeltas can't unmarshal raw deltas, cleaning out file")
			if err2 := s.cleanPluginDeltas(reapedPlugins, entityKey); err2 != nil {
				llog.WithError(err2).Error("can't clean plugin deltas")
			}
			return nil, err
		}
		deltas = append(deltas, deltasGroup)
	}
	return deltas, nil
}

func (s *Store) ChangeDefaultEntity(newEntityKey string) {
	s.defaultEntityKey = newEntityKey
}

// entityFolder provides the folder name for a given entity ID, or for the agent default entity in case entityKey is an
// empty string
func (s *Store) entityFolder(entityKey string) string {
	if entityKey == "" || entityKey == s.defaultEntityKey {
		return localEntityFolder
	}
	return helpers.SanitizeFileName(entityKey)
}

// RemoveEntity removes the entity cached storage.
func (s *Store) RemoveEntity(entityKey string) error {
	return s.RemoveEntityFolders(s.entityFolder(entityKey))
}

// RemoveEntityFolders removes the entity cached storage from the entities whose folder is equal to the argument.
func (s *Store) RemoveEntityFolders(entityFolder string) error {
	errStrings := s.removeEntityEntries(s.DataDir, entityFolder)
	errStrings = append(errStrings, s.removeEntityEntries(s.CacheDir, entityFolder)...)
	if len(errStrings) > 0 {
		return fmt.Errorf("errors happened while removing entity folders: %s", strings.Join(errStrings, ", "))
	}
	return nil
}

func (s *Store) removeEntityEntries(dir, entityFolder string) (errStrings []string) {
	errStrings = make([]string, 0)
	// For all the plugins in the given directory
	plugins, err := ioutil.ReadDir(dir)
	if err != nil {
		errStrings = append(errStrings, err.Error())
		return errStrings
	}
	for _, plugin := range plugins {
		if plugin.IsDir() && !nonEntityFolders[plugin.Name()] {
			// For all the entities under the plugin folder, remove those whose directory name is planned for removal
			entityPath := filepath.Join(dir, plugin.Name(), entityFolder)
			if _, err := os.Stat(entityPath); err == nil {
				helpers.DebugStackf("removing: %s", entityPath)
				if err = os.RemoveAll(entityPath); err != nil {
					errStrings = append(errStrings, err.Error())
				}
			}
		}
	}
	return errStrings
}

// ScanEntityFolders returns a set of those entities that have been found in the different plugin folders.
func (s *Store) ScanEntityFolders() (map[string]interface{}, error) {
	entities, err := s.fetchEntities(s.DataDir)
	if err != nil {
		return nil, err
	}
	cacheEntities, err := s.fetchEntities(s.CacheDir)
	if err != nil {
		return entities, err
	}
	for entity := range cacheEntities {
		entities[entity] = true
	}
	return entities, nil
}

func (s *Store) fetchEntities(dir string) (map[string]interface{}, error) {
	entities := make(map[string]interface{})

	// For all the plugins in the given directory
	plugins, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, plugin := range plugins {
		if plugin.IsDir() && !nonEntityFolders[plugin.Name()] {
			// For all the entities under the plugin folder, adds them to the map
			entityFolders, err := ioutil.ReadDir(filepath.Join(dir, plugin.Name()))
			if err != nil {
				return entities, err
			}
			for _, folder := range entityFolders {
				if folder.IsDir() {
					entities[folder.Name()] = true
				}
			}
		}
	}
	return entities, nil
}

// getPluginDelta returns the difference between the source inventory
// json and the cache inventory json of the given plugin from the given
// entity. If there is no difference, then an empty JSON object is
// retured `{}`.
func (s *Store) newPluginDelta(pluginItem *PluginInfo, entityKey string) (delta, error) {
	sourceFilePath := s.SourceFilePath(pluginItem, entityKey)
	sourceB, err := ioutil.ReadFile(sourceFilePath)
	if err != nil {
		slog.WithFields(logrus.Fields{
			"entityKey": entityKey,
			"plugin":    pluginItem.ID(),
		}).WithError(err).Error("can't read inventory source")
		return delta{}, err
	}

	cacheFilePath := s.cachedFilePath(pluginItem, entityKey)
	_, err = os.Stat(cacheFilePath)
	if os.IsNotExist(err) {
		return delta{value: sourceB, full: true}, nil
	}

	cacheB, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		slog.WithError(err).Error("can't read inventory cache")
		return delta{}, err
	}

	del, err := s.getDeltaFromJSON(cacheB, sourceB)
	return delta{value: del, full: false}, err
}

// updatePluginInventoryCache updates the inventory cache file of the
// given entity plugin. First it generates the delta with regard to the
// current source file, if the delta is not empty, stores it and updates
// the cache file.
//
// Returns false when the delta is empty, meaning that the plugin state
// didn't change; otherwise, it returns trues.
func (s *Store) updatePluginInventoryCache(pi *PluginInfo, entityKey string) (updated bool, err error) {

	updated = true

	llog := slog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{"entityKey": entityKey, "plugin": pi.ID()}
	})

	del, err := s.newPluginDelta(pi, entityKey)
	if err != nil {
		llog.WithError(err).Error("can't calculate delta from JSON files")
		// Corrupted JSON. Removing plugin folder and deltas
		if err := s.clearPluginDeltaStore(pi, entityKey); err != nil {
			llog.WithError(err).Warn("can't clear plugin delta store")
		}
		if err := os.RemoveAll(s.SourceFilePath(pi, entityKey)); err != nil {
			llog.WithError(err).Warn("can't remove source file path")
		}
		return
	}

	if bytes.Equal(EMPTY_DELTA, del.value) {
		updated = false
		return
	}

	trace.AttrOn(
		func() bool { return ids.CustomAttrsID.String() == pi.ID() },
		"reap change, item: %+v", *pi,
	)

	err = s.storeDelta(pi, entityKey, del)
	if err != nil {
		llog.WithError(err).Error("can't commit inventory")
	}

	err = s.replacePluginCacheFileWithSource(pi, entityKey)
	if err != nil {
		llog.WithError(err).Error("replacing plugin cache file failed")
	}

	return
}

// replacePluginCacheFileWithSource replaces the given entity plugin
// inventory cache file with the source file.  It deletes the cache file
// if it already exists.
func (s *Store) replacePluginCacheFileWithSource(pluginItem *PluginInfo, entityKey string) error {
	sourceFilePath := s.SourceFilePath(pluginItem, entityKey)
	cachedFilePath := s.cachedFilePath(pluginItem, entityKey)
	return helpers.CopyFile(sourceFilePath, cachedFilePath)
}

// UpdatePluginsInventoryCache looks for all the plugins of the given
// entityKey located in the store DataDir, for each of the plugins, it
// compares the inventory json source and compares it against the
// cached json.
//
// If the JSONs differ, creates a delta file and replaces the cache
// with the source file. Then finally writes on disk the plugin ID maps.
func (s *Store) UpdatePluginsInventoryCache(entityKey string) (err error) {
	activePlugins, err := s.collectPluginFiles(
		s.DataDir,
		entityKey,
		helpers.JsonFilesRegexp,
	)
	if err != nil {
		slog.WithFields(logrus.Fields{
			"entityKey": entityKey,
			"path":      s.DataDir,
		}).WithError(err).Error("can't get plugins in the data directory")
		return
	}

	var saveState = false
	var pUpdated bool
	var pErr error
	for _, pluginItem := range activePlugins {
		pUpdated, pErr = s.updatePluginInventoryCache(pluginItem, entityKey)
		if pErr != nil {
			slog.WithFields(logrus.Fields{
				"entityKey": entityKey,
				"plugin":    pluginItem.ID(),
			}).WithError(err).Error("error updating plugin inventory")
		}
		saveState = saveState || pUpdated
	}

	if !saveState {
		return
	}

	err = s.SaveState()
	if err != nil {
		slog.WithField("entityKey", entityKey).WithError(err).Error("error flushing inventory to cache")
	}
	return
}

// StorePluginOutput will take a PluginOutput blob and write it to the
// data directory in JSON format
func (s *Store) SavePluginSource(entityKey, category, term string, source map[string]interface{}) (err error) {
	// construct the plugin data directory and ensure it exists
	outputDir := s.PluginDirPath(category, entityKey)
	if err = disk.MkdirAll(outputDir, DATA_DIR_MODE); err != nil {
		return err
	}

	// construct the output file path
	outputFile := fmt.Sprintf("%s/%s.json", outputDir, term)

	sourceB, err := json.Marshal(source)
	if err != nil {
		return
	}

	if bytes.Contains(sourceB, NULL) {
		sourceB, err = removeNilsFromMarshaledJSON(sourceB)
		if err != nil {
			return
		}
	}

	if len(sourceB) > s.maxInventorySize {
		err = fmt.Errorf(
			"Plugin data for entity %v plugin %v/%v is larger than max size of %v",
			entityKey,
			category,
			term,
			s.maxInventorySize,
		)
		return
	}
	err = disk.WriteFile(outputFile, sourceB, DATA_FILE_MODE)
	return
}
