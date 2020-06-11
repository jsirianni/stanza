package builtin

import (
	"context"
	"testing"
	"time"

	"github.com/bluemedora/bplogagent/entry"
	"github.com/bluemedora/bplogagent/plugin/helper"
	"github.com/bluemedora/bplogagent/plugin/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {

	testCases := []struct {
		name           string
		stample        string
		gotimeLayout   string
		strptimeLayout string
	}{
		{
			name:           "unix",
			stample:        "Mon Jan 2 15:04:05 MST 2006",
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
		},
		{
			name:           "almost-unix",
			stample:        "Mon Jan 02 15:04:05 MST 2006",
			gotimeLayout:   "Mon Jan 02 15:04:05 MST 2006",
			strptimeLayout: "%a %b %d %H:%M:%S %Z %Y",
		},
		{
			name:           "kitchen",
			stample:        "12:34PM",
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
		},
		{
			name:           "countdown",
			stample:        "-0100 01 01 01 01 01 01",
			gotimeLayout:   "-0700 06 05 04 03 02 01",
			strptimeLayout: "%z %y %S %M %H %e %m",
		},
		{
			name:           "debian-syslog",
			stample:        "Jun 09 11:39:45",
			gotimeLayout:   "Jan 02 15:04:05",
			strptimeLayout: "%b %d %H:%M:%S",
		},
		{
			name:           "opendistro",
			stample:        "2020-06-09T15:39:58",
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
		},
		{
			name:           "postgres",
			stample:        "2019-11-05 10:38:35.118 EST",
			gotimeLayout:   "2006-01-02 15:04:05.999 MST",
			strptimeLayout: "%Y-%m-%d %H:%M:%S.%L %Z",
		},
		{
			name:           "ibm-mq",
			stample:        "3/4/2018 11:52:29",
			gotimeLayout:   "1/2/2006 15:04:05",
			strptimeLayout: "%q/%g/%Y %H:%M:%S",
		},
		{
			name:           "cassandra",
			stample:        "2019-11-27T09:34:32.901-0500",
			gotimeLayout:   "2006-01-02T15:04:05.999-0700",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%L%z",
		},
		{
			name:           "oracle",
			stample:        "2019-10-15T10:42:01.900436-04:00",
			gotimeLayout:   "2006-01-02T15:04:05.999999-07:00",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%f%j",
		},
		{
			name:           "oracle-listener",
			stample:        "22-JUL-2019 15:16:13",
			gotimeLayout:   "02-Jan-2006 15:04:05",
			strptimeLayout: "%d-%b-%Y %H:%M:%S",
		},
		{
			name:           "k8s",
			stample:        "2019-03-08T18:41:12.152531115Z",
			gotimeLayout:   "2006-01-02T15:04:05.999999999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%sZ",
		},
		{
			name:           "jetty",
			stample:        "05/Aug/2019:20:38:46 +0000",
			gotimeLayout:   "02/Jan/2006:15:04:05 -0700",
			strptimeLayout: "%d/%b/%Y:%H:%M:%S %z",
		},
		{
			name:           "puppet",
			stample:        "Aug  4 03:26:02",
			gotimeLayout:   "Jan _2 15:04:05",
			strptimeLayout: "%b %e %H:%M:%S",
		},
	}

	rootField := entry.NewField()
	someField := entry.NewField("some_field")
	makeTestEntry := func(field entry.Field, value string) *entry.Entry {
		e := entry.New()
		e.Set(field, value)
		return e
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := time.Parse(tc.gotimeLayout, tc.stample)
			require.NoError(t, err, "Test configuration includes invalid timestamp or layout")

			gotimeRootCfg := parseTimeTestConfig(gotimeKey, tc.gotimeLayout, rootField)
			t.Run("gotime-root", runTest(t, gotimeRootCfg, makeTestEntry(rootField, tc.stample), expected))

			gotimeNonRootCfg := parseTimeTestConfig(gotimeKey, tc.gotimeLayout, someField)
			t.Run("gotime-non-root", runTest(t, gotimeNonRootCfg, makeTestEntry(someField, tc.stample), expected))

			strptimeRootCfg := parseTimeTestConfig(strptimeKey, tc.strptimeLayout, rootField)
			t.Run("strptime-root", runTest(t, strptimeRootCfg, makeTestEntry(rootField, tc.stample), expected))

			strptimeNonRootCfg := parseTimeTestConfig(strptimeKey, tc.strptimeLayout, someField)
			t.Run("strptime-non-root", runTest(t, strptimeNonRootCfg, makeTestEntry(someField, tc.stample), expected))
		})
	}
}

func runTest(t *testing.T, cfg *TimeParserConfig, ent *entry.Entry, expected time.Time) func(*testing.T) {

	return func(t *testing.T) {
		buildContext := testutil.NewTestBuildContext(t)

		gotimePlugin, err := cfg.Build(buildContext)
		require.NoError(t, err)

		mockOutput := &testutil.Plugin{}
		resultChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			resultChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		timeParser := gotimePlugin.(*TimeParser)
		timeParser.Output = mockOutput

		require.NoError(t, timeParser.Process(context.Background(), ent))

		select {
		case e := <-resultChan:
			require.Equal(t, expected, e.Timestamp)
		case <-time.After(time.Second):
			require.FailNow(t, "Timed out waiting for entry to be processed")
		}
	}
}

func parseTimeTestConfig(flavor, layout string, parseFrom entry.Field) *TimeParserConfig {
	return &TimeParserConfig{
		TransformerConfig: helper.TransformerConfig{
			BasicConfig: helper.BasicConfig{
				PluginID:   "test_plugin_id",
				PluginType: "time_parser",
			},
			OutputID: "output1",
		},
		LayoutFlavor: flavor,
		Layout:       layout,
		ParseFrom:    parseFrom,
	}
}
