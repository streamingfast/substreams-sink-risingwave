package main

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	. "github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/logging"
	sink "github.com/streamingfast/substreams-sink"
	pbsubstreamsrpc "github.com/streamingfast/substreams/pb/sf/substreams/rpc/v2"
	"go.uber.org/zap"
)

// Injected at build time
var version = ""

var zlog, tracer = logging.RootLogger("project", "github.com/streamingfast/substreams-sink-risingwave")

func main() {
	logging.InstantiateLoggers()

	Run(
		"substreams-sink-risingwave <endpoint> </path/to/spkg> <output_module> [startblock[:endblock]]",
		"Always runs in development mode",

		Execute(run),

		RangeArgs(3, 4),
		Description(`
			Sink data from a Substreams to RisingWave using Google PubSub
		`),
		Example(`
			substreams-sink-risingwave mainnet.sol.streamingfast.io:443 https://spkg.io/v1/packages/tl_account_sol_balances_1_0_0/v1.0.0 map_block 313000000:313000010 --project dev --topic test
		`),

		Flags(func(flags *pflag.FlagSet) {
			sink.AddFlagsToSet(flags, sink.FlagIgnore(sink.FlagDevelopmentMode))
			flags.String("project", "dev", "Google Cloud Project ID")
			flags.String("topic", "test", "Google PubSub Topic")
			flags.Bool(sink.FlagDevelopmentMode, true, "Enable development mode, should not be used for production workload")
		}),

		ConfigureVersion(version),
		ConfigureViper("SUBSTREAMS_SINK_RISINGWAVE"),
		OnCommandErrorLogAndExit(zlog),
	)

}

func run(cmd *cobra.Command, args []string) error {
	zlog.Info("Executing sink")

	ctx := cmd.Context()
	endpoint := args[0]
	manifestPath := args[1]
	outputModuleName := args[2]
	blockRange := ""
	if len(args) > 3 {
		blockRange = args[3]
	}
	sinker, err := sink.NewFromViper(cmd, sink.IgnoreOutputModuleType, endpoint, manifestPath, outputModuleName, blockRange, zlog, tracer)
	if err != nil {
		return err
	}

	pubsubSinker, err := newPubsubSink(ctx, sflags.MustGetString(cmd, "project"), sflags.MustGetString(cmd, "topic"), zlog)
	if err != nil {
		return err
	}

	sinker.Run(ctx, nil, pubsubSinker)

	return nil
}

func newPubsubSink(ctx context.Context, project, topic string, logger *zap.Logger) (*pubsubSink, error) {

	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return nil, err
	}
	t := client.Topic(topic)

	return &pubsubSink{
		topic:  t,
		logger: logger,
	}, nil
}

type pubsubSink struct {
	topic  *pubsub.Topic
	logger *zap.Logger
}

func (s *pubsubSink) HandleBlockScopedData(ctx context.Context, data *pbsubstreamsrpc.BlockScopedData, isLive *bool, cursor *sink.Cursor) error {
	// - `ctx` is the context runtime, your handler should be minimal, so normally you shouldn't use this.
	// - `undoSignal` contains the last valid block that is still valid, any data saved after this last saved block should be discarded.
	// - `cursor` is the cursor at the given block, this cursor should be saved regularly as a checkpoint in case the process is interrupted.

	b, err := json.Marshal(data.Clock)
	if err != nil {
		return err
	}
	msg := &pubsub.Message{
		ID:   string(b),
		Data: data.Output.MapOutput.Value,
	}
	fmt.Printf("getting msg %+v\n", msg.ID)
	res := s.topic.Publish(ctx, msg)
	_ = res
	//id, err := res.Get(ctx) // this is blocking ... need to optimize maybe ?
	//if err != nil {
	//	s.logger.Warn("error publishing message", zap.String("id", id), zap.Error(err))
	//	return err
	//}
	s.logger.Debug("published message", zap.String("id", msg.ID))
	return nil

}
func (s *pubsubSink) HandleBlockUndoSignal(ctx context.Context, undoSignal *pbsubstreamsrpc.BlockUndoSignal, cursor *sink.Cursor) error {
	return nil
}
