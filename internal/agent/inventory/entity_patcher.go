package inventory

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

type PatchSenderProviderFunc func(entity.Entity) (PatchSender, error)

type EntityPatcher struct {
	BasePatcher
	entities map[entity.Key]struct {
		sender       PatchSender
		needsReaping bool
	}
	patchSenderProviderFn PatchSenderProviderFunc
}

func NewEntityPatcher(cfg PatcherConfig, deltaStore *delta.Store, patchSenderProviderFn PatchSenderProviderFunc) Patcher {
	return &EntityPatcher{
		BasePatcher: BasePatcher{
			deltaStore: deltaStore,
			cfg:        cfg,
		},
		entities: map[entity.Key]struct {
			sender       PatchSender
			needsReaping bool
		}{},
		patchSenderProviderFn: patchSenderProviderFn,
	}
}

func (ep *EntityPatcher) Send() error {
	for _, inventory := range ep.entities {
		err := inventory.sender.Process()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ep *EntityPatcher) Reap() {
	for key := range ep.entities {
		ep.reapEntity(key)
	}
}

func (ep *EntityPatcher) Save(data types.PluginOutput) error {
	if data.NotApplicable {
		return nil
	}

	if err := ep.registerEntity(data.Entity); err != nil {
		return fmt.Errorf("failed to save plugin inventory data, error: %w", err)
	}

	if err := ep.BasePatcher.save(data); err != nil {
		return fmt.Errorf("failed to save plugin inventory data, error: %w", err)
	}

	e := ep.entities[data.Entity.Key]
	e.needsReaping = true
	return nil
}

func (ep *EntityPatcher) registerEntity(entity entity.Entity) error {
	if _, found := ep.entities[entity.Key]; found {
		return nil
	}

	ilog.WithField("entityKey", entity.Key.String()).
		WithField("entityID", entity.ID).Debug("Registering inventory for entity.")

	sender, err := ep.patchSenderProviderFn(entity)
	if err != nil {
		return fmt.Errorf("failed to register inventory for entity: %s, %v", entity.Key, err)
	}

	ep.entities[entity.Key] = struct {
		sender       PatchSender
		needsReaping bool
	}{sender: sender, needsReaping: true}

	return nil
}
