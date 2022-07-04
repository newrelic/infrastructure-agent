package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Config keeps the required configuration for the action.
type Config struct {
	AWSRegion                string
	ECSClusterName           string
	TaskDefinitionName       string
	ContainerMakeTarget      string
	AWSVpcSubnet             string
	CloudWatchLogsGroupName  string
	CloudWatchLogsStreamName string
	MaxLogLines              int
	TimeoutMillis            int
}

const (
	defaultTimeoutMillis = 240000
	defaultMaxLogLines   = 200

	logLinesReqBackoff = 5 * time.Second
)

func LoadConfig() Config {
	viper.BindEnv("aws_region")
	viper.BindEnv("ecs_cluster_name")
	viper.BindEnv("task_definition_name")
	viper.BindEnv("container_make_target")
	viper.BindEnv("aws_vpc_subnet")
	viper.BindEnv("cloud_watch_logs_group_name")
	viper.BindEnv("cloud_watch_logs_stream_name")
	viper.BindEnv("timeout_millis")
	viper.BindEnv("max_log_lines")

	timeoutMillis := viper.GetInt("timeout_millis")
	if timeoutMillis == 0 {
		timeoutMillis = defaultTimeoutMillis
	}

	maxLogLines := viper.GetInt("max_log_lines")
	if maxLogLines == 0 {
		maxLogLines = defaultMaxLogLines
	}

	return Config{
		AWSRegion:                viper.GetString("aws_region"),
		ECSClusterName:           viper.GetString("ecs_cluster_name"),
		TaskDefinitionName:       viper.GetString("task_definition_name"),
		ContainerMakeTarget:      viper.GetString("container_make_target"),
		AWSVpcSubnet:             viper.GetString("aws_vpc_subnet"),
		CloudWatchLogsGroupName:  viper.GetString("cloud_watch_logs_group_name"),
		CloudWatchLogsStreamName: viper.GetString("cloud_watch_logs_stream_name"),
		TimeoutMillis:            timeoutMillis,
		MaxLogLines:              maxLogLines,
	}
}

func main() {
	params := LoadConfig()
	taskRunner, cfg := prepareFargateTask(params)

	timeout := time.Duration(params.TimeoutMillis) * time.Millisecond
	id, err := runFargateTask(timeout, taskRunner)
	if err != nil {
		log.Fatalf("failed to run task: %v", err)
	}

	// to be able to add timeout later
	ctx := context.Background()

	printFargateTaskLogs(ctx, params, cfg, taskRunner, id)
}

func prepareFargateTask(params Config) (*TaskRunner, aws.Config) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
	)
	if err != nil {
		log.Fatalf("failed: %v", err)
	}

	taskSpecs := &ecs.RunTaskInput{
		Cluster:        &params.ECSClusterName,
		TaskDefinition: &params.TaskDefinitionName,
		LaunchType:     ecsTypes.LaunchTypeFargate,

		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				Subnets: []string{params.AWSVpcSubnet},
			},
		},
	}

	// modify task input if command is specified
	if params.ContainerMakeTarget != "" {
		taskDefinition := &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &params.TaskDefinitionName,
		}

		containerOverride := NewContainerOverride(taskDefinition, ecs.NewFromConfig(cfg))

		ctx, cancelFn := context.WithTimeout(context.Background(), time.Duration(params.TimeoutMillis)*time.Millisecond)
		defer cancelFn()

		containers, err := containerOverride.GetContainersOverride(ctx, params.ContainerMakeTarget)
		if err != nil {
			log.Fatalf("failed: %v", err)
		}
		taskSpecs.Overrides = &ecsTypes.TaskOverride{
			ContainerOverrides: containers,
		}
	}

	return NewTaskRunner(taskSpecs, ecs.NewFromConfig(cfg)), cfg
}

func runFargateTask(timeout time.Duration, taskRunner *TaskRunner) (string, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	// TODO: https://github.com/newrelic/infrastructure-agent/issues/1248
	taskOutput, err := taskRunner.Run(ctx)
	if err != nil {
		log.Fatalf("failed to run task: %v", err)
	}

	id, err := getTaskID(taskOutput)
	if err != nil {
		log.Fatalf("failed to configure log tailer: %v", err)
	}
	return id, nil
}

func printFargateTaskLogs(ctx context.Context, params Config, cfg aws.Config, taskRunner *TaskRunner, id string) {
	logTailerConfig := CloudWatchLogTailerConfig{
		LogGroupName:  params.CloudWatchLogsGroupName,
		LogStreamName: fmt.Sprintf("%s/%s", params.CloudWatchLogsStreamName, id),
		MaxLines:      params.MaxLogLines,
	}

	logTailer := NewCloudWatchLogTailer(logTailerConfig, cloudwatchlogs.NewFromConfig(cfg))

	for {
		logs, err := logTailer.GetLogs(ctx)
		if err != nil {
			log.Fatalf("failed to read logs: %v", err)
		}
		for _, line := range logs {
			log.Printf("%s\n", *line.Message)
		}

		finished, exitCode, err := taskRunner.IsFinished()
		if err != nil {
			log.Fatalf("failed to check if task has finished: %v", err)
		}

		if finished {
			os.Exit(exitCode)
		}

		if len(logs) == 0 {
			time.Sleep(logLinesReqBackoff)
		}
	}
}

// ContainerOverride returns a list of containers definition with an override command
type ContainerOverride struct {
	specs     *ecs.DescribeTaskDefinitionInput
	awsClient *ecs.Client
}

// NewContainerOverride returns a new ContainerOverride.
func NewContainerOverride(taskDefinition *ecs.DescribeTaskDefinitionInput, awsClient *ecs.Client) *ContainerOverride {
	return &ContainerOverride{
		specs:     taskDefinition,
		awsClient: awsClient,
	}
}

// GetContainerOverride returns a container configuration with a new command
func (co *ContainerOverride) GetContainersOverride(ctx context.Context, command string) ([]ecsTypes.ContainerOverride, error) {
	var err error
	task, err := co.awsClient.DescribeTaskDefinition(ctx, co.specs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe task definitions")
	}

	containerOverrides := make([]ecsTypes.ContainerOverride, len(task.TaskDefinition.ContainerDefinitions))

	for i, container := range task.TaskDefinition.ContainerDefinitions {
		containerOverrides[i] = ecsTypes.ContainerOverride{
			Name:                 container.Name,
			Environment:          container.Environment,
			EnvironmentFiles:     container.EnvironmentFiles,
			Command:              []string{command},
			ResourceRequirements: container.ResourceRequirements,
		}
	}

	return containerOverrides, nil
}

// TaskRunner runs a new task based on provided specs.
type TaskRunner struct {
	specs       *ecs.RunTaskInput
	awsClient   *ecs.Client
	runningTask *ecs.RunTaskOutput
}

// NewTaskRunner returns a new TaskRunner.
func NewTaskRunner(taskSpecs *ecs.RunTaskInput, awsClient *ecs.Client) *TaskRunner {
	return &TaskRunner{
		specs:     taskSpecs,
		awsClient: awsClient,
	}
}

// Run starts ecs task and waits for it to be in running state.
func (tr *TaskRunner) Run(ctx context.Context) (*ecs.DescribeTasksOutput, error) {
	var err error
	tr.runningTask, err = tr.awsClient.RunTask(ctx, tr.specs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run task")
	}

	log.Println("Waiting for task to run...")

	defer func() {
		log.Println("Task is running!")
	}()

	return tr.WaitForStatus(ctx, "RUNNING")
}

// GetStatus pulls ecs task status.
func (tr *TaskRunner) GetStatus(ctx context.Context) (*ecs.DescribeTasksOutput, error) {
	if tr.runningTask == nil ||
		len(tr.runningTask.Tasks) == 0 ||
		tr.runningTask.Tasks[0].TaskArn == nil {

		return nil, fmt.Errorf("task not started")
	}

	taskArn := *tr.runningTask.Tasks[0].TaskArn

	specs := &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: tr.specs.Cluster,
	}

	return tr.awsClient.DescribeTasks(ctx, specs)
}

// WaitForStatus pulls ecs task until desired state is reached or ctx canceled.
func (tr *TaskRunner) WaitForStatus(ctx context.Context, status string) (*ecs.DescribeTasksOutput, error) {
	for {
		select {
		case <-time.After(time.Second):
			output, err := tr.GetStatus(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get status while waiting for task")
			}

			if len(output.Tasks) > 0 && output.Tasks[0].LastStatus != nil {
				lastStatus := *output.Tasks[0].LastStatus
				if lastStatus == status {
					return output, nil
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// IsFinished checks if the ecs task has finised and returns the exit code of the container.
func (tr *TaskRunner) IsFinished() (finished bool, exitCode int, err error) {
	exitCode = -1
	// TODO: pass context && should retry this?
	status, err := tr.GetStatus(context.Background())
	if err != nil {
		err = errors.Wrap(err, "failed to get task status while checking if has finished")
		return
	}

	if status == nil || len(status.Tasks) == 0 {
		err = errors.New("failed to get task status, empty status/tasks")
		return
	}

	task := (*status).Tasks[0]

	if task.LastStatus != nil && *task.LastStatus == "STOPPED" {
		finished = true

		if len(task.Containers) > 0 && task.Containers[0].ExitCode != nil {
			exitCode = int(*status.Tasks[0].Containers[0].ExitCode)
		}
	}

	return
}

// CloudWatchLogTailerConfig configures CloudWatchLogTailer.
type CloudWatchLogTailerConfig struct {
	LogGroupName  string
	LogStreamName string
	MaxLines      int
}

// CloudWatchLogTailer is used to fetch logs from cloudwatchlogs service.
type CloudWatchLogTailer struct {
	config    CloudWatchLogTailerConfig
	awsClient *cloudwatchlogs.Client
	nextToken string
}

// NewCloudWatchLogTailer returns new CloudWatchLogTailer.
func NewCloudWatchLogTailer(config CloudWatchLogTailerConfig, awsClient *cloudwatchlogs.Client) *CloudWatchLogTailer {
	return &CloudWatchLogTailer{
		config:    config,
		awsClient: awsClient,
	}
}

// GetLogs returns the latest log lines available in the configured cloudwatchlogs.
func (c *CloudWatchLogTailer) GetLogs(ctx context.Context) ([]types.OutputLogEvent, error) {
	cfg := &cloudwatchlogs.GetLogEventsInput{
		Limit:         aws.Int32(int32(c.config.MaxLines)),
		LogGroupName:  aws.String(c.config.LogGroupName),
		LogStreamName: aws.String(c.config.LogStreamName),
		StartFromHead: aws.Bool(true),
	}

	if c.nextToken != "" {
		cfg.NextToken = aws.String(c.nextToken)
	}

	logEventsResp, err := c.awsClient.GetLogEvents(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// GetLogEvents can return empty results while there are more log events available through the token.
	if len(logEventsResp.Events) > 0 {
		// If you have reached the end of the stream, it returns the same token you passed in.
		c.nextToken = *logEventsResp.NextForwardToken
	}

	return logEventsResp.Events, nil
}

// getTaskID will check the ecs.DescribeTasksOutput object and extract the id we use to build
// the LogStreamName from taskArn.
func getTaskID(taskOutput *ecs.DescribeTasksOutput) (string, error) {
	if taskOutput == nil ||
		len(taskOutput.Tasks) == 0 ||
		taskOutput.Tasks[0].TaskArn == nil {
		return "", errors.New("failed to get task id from empty task")
	}

	taskArn := *taskOutput.Tasks[0].TaskArn

	i := strings.LastIndex(taskArn, "/")
	if i < 0 {
		return "", fmt.Errorf("failed to get task id, bad taskArn format: '%s'", taskArn)
	}

	return taskArn[i+1:], nil
}
