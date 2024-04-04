package fs

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Global
var (
	// globalConfig for rclone
	globalConfig = NewConfig()

	// Read a value from the config file
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	ConfigFileGet = func(section, key string) (string, bool) { return "", false }

	// Set a value into the config file and persist it
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	ConfigFileSet = func(section, key, value string) (err error) {
		return errors.New("no config file set handler")
	}

	// CountError counts an error.  If any errors have been
	// counted then rclone will exit with a non zero error code.
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	CountError = func(err error) error { return err }

	// ConfigProvider is the config key used for provider options
	ConfigProvider = "provider"
)

// ConfigInfo is filesystem config options
type ConfigInfo struct {
	LogLevel               LogLevel      `yaml:"log_level"`
	StatsLogLevel          LogLevel      `yaml:"stats_log_level"`
	UseJSONLog             bool          `yaml:"use_json_log"`
	DryRun                 bool          `yaml:"dry_run"`
	Interactive            bool          `yaml:"interactive"`
	CheckSum               bool          `yaml:"check_sum"`
	SizeOnly               bool          `yaml:"size_only"`
	IgnoreTimes            bool          `yaml:"ignore_times"`
	IgnoreExisting         bool          `yaml:"ignore_existing"`
	IgnoreErrors           bool          `yaml:"ignore_errors"`
	ModifyWindow           time.Duration `yaml:"modify_window"`
	Checkers               int           `yaml:"checkers"`
	Transfers              int           `yaml:"transfers"`
	ConnectTimeout         time.Duration `yaml:"connect_timeout"` // Connect timeout
	Timeout                time.Duration `yaml:"timeout"`         // Data channel timeout
	ExpectContinueTimeout  time.Duration `yaml:"expect_continue_timeout"`
	Dump                   DumpFlags     `yaml:"dump"`
	InsecureSkipVerify     bool          `yaml:"insecure_skip_verify"` // Skip server certificate verification
	DeleteMode             DeleteMode    `yaml:"delete_mode"`
	MaxDelete              int64         `yaml:"max_delete"`
	TrackRenames           bool          `yaml:"track_renames"`          // Track file renames
	TrackRenamesStrategy   string        `yaml:"track_renames_strategy"` // Comma separated list of strategies used to track renames
	LowLevelRetries        int           `yaml:"low_level_retries"`
	UpdateOlder            bool          `yaml:"update_older"` // Skip files that are newer on the destination
	NoGzip                 bool          `yaml:"no_gzip"`      // Disable compression
	MaxDepth               int           `yaml:"max_depth"`
	IgnoreSize             bool          `yaml:"ignore_size"`
	IgnoreChecksum         bool          `yaml:"ignore_checksum"`
	IgnoreCaseSync         bool          `yaml:"ignore_case_sync"`
	NoTraverse             bool          `yaml:"no_traverse"`
	CheckFirst             bool          `yaml:"check_first"`
	NoCheckDest            bool          `yaml:"no_check_dest"`
	NoUnicodeNormalization bool          `yaml:"no_unicode_normalization"`
	NoUpdateModTime        bool          `yaml:"no_update_mod_time"`
	DataRateUnit           string        `yaml:"data_rate_unit"`
	CompareDest            string        `yaml:"compare_dest"`
	CopyDest               string        `yaml:"copy_dest"`
	BackupDir              string        `yaml:"backup_dir"`
	Suffix                 string        `yaml:"suffix"`
	SuffixKeepExtension    bool          `yaml:"suffix_keep_extension"`
	UseListR               bool          `yaml:"use_list_r"`
	BufferSize             SizeSuffix    `yaml:"buffer_size"`
	BwLimit                BwTimetable   `yaml:"bw_limit"`
	BwLimitFile            BwTimetable   `yaml:"bw_limit_file"`
	TPSLimit               float64       `yaml:"tps_limit"`
	TPSLimitBurst          int           `yaml:"tps_limit_burst"`
	BindAddr               net.IP        `yaml:"bind_addr"`
	DisableFeatures        []string      `yaml:"disable_features"`
	UserAgent              string        `yaml:"user_agent"`
	Immutable              bool          `yaml:"immutable"`
	AutoConfirm            bool          `yaml:"auto_confirm"`
	StreamingUploadCutoff  SizeSuffix    `yaml:"streaming_upload_cutoff"`
	StatsFileNameLength    int           `yaml:"stats_file_name_length"`
	AskPassword            bool          `yaml:"ask_password"`
	PasswordCommand        SpaceSepList  `yaml:"password_command"`
	UseServerModTime       bool          `yaml:"use_server_mod_time"`
	MaxTransfer            SizeSuffix    `yaml:"max_transfer"`
	MaxDuration            time.Duration `yaml:"max_duration"`
	CutoffMode             CutoffMode    `yaml:"cutoff_mode"`
	MaxBacklog             int           `yaml:"max_backlog"`
	MaxStatsGroups         int           `yaml:"max_stats_groups"`
	StatsOneLine           bool          `yaml:"stats_one_line"`
	StatsOneLineDate       bool          `yaml:"stats_one_line_date"`        // If we want a date prefix at all
	StatsOneLineDateFormat string        `yaml:"stats_one_line_date_format"` // If we want to customize the prefix
	ErrorOnNoTransfer      bool          `yaml:"error_on_no_transfer"`       // Set appropriate exit code if no files transferred
	Progress               bool          `yaml:"progress"`
	ProgressTerminalTitle  bool          `yaml:"progress_terminal_title"`
	Cookie                 bool          `yaml:"cookie"`
	UseMmap                bool          `yaml:"use_mmap"`
	CaCert                 string        `yaml:"ca_cert"`     // Client Side CA
	ClientCert             string        `yaml:"client_cert"` // Client Side Cert
	ClientKey              string        `yaml:"client_key"`  // Client Side Key
	MultiThreadCutoff      SizeSuffix    `yaml:"multi_thread_cutoff"`
	MultiThreadStreams     int           `yaml:"multi_thread_streams"`
	MultiThreadSet         bool          `yaml:"multi_thread_set"` // whether MultiThreadStreams was set (set in fs/config/configflags)
	OrderBy                string        `yaml:"order_by"`         // instructions on how to order the transfer
	UploadHeaders          []*HTTPOption `yaml:"upload_headers"`
	DownloadHeaders        []*HTTPOption `yaml:"download_headers"`
	Headers                []*HTTPOption `yaml:"headers"`
	RefreshTimes           bool          `yaml:"refresh_times"`
	NoConsole              bool          `yaml:"no_console"`
}

// NewConfig creates a new config with everything set to the default
// value.  These are the ultimate defaults and are overridden by the
// config module.
func NewConfig() *ConfigInfo {
	c := new(ConfigInfo)

	// Set any values which aren't the zero for the type
	c.LogLevel = LogLevelNotice
	c.StatsLogLevel = LogLevelInfo
	c.ModifyWindow = time.Nanosecond
	c.Checkers = 8
	c.Transfers = 4
	c.ConnectTimeout = 60 * time.Second
	c.Timeout = 5 * 60 * time.Second
	c.ExpectContinueTimeout = 1 * time.Second
	c.DeleteMode = DeleteModeDefault
	c.MaxDelete = -1
	c.LowLevelRetries = 10
	c.MaxDepth = -1
	c.DataRateUnit = "bytes"
	c.BufferSize = SizeSuffix(16 << 20)
	c.UserAgent = "rclone/" + Version
	c.StreamingUploadCutoff = SizeSuffix(100 * 1024)
	c.MaxStatsGroups = 1000
	c.StatsFileNameLength = 45
	c.AskPassword = true
	c.TPSLimitBurst = 1
	c.MaxTransfer = -1
	c.MaxBacklog = 10000
	// We do not want to set the default here. We use this variable being empty as part of the fall-through of options.
	//	c.StatsOneLineDateFormat = "2006/01/02 15:04:05 - "
	c.MultiThreadCutoff = SizeSuffix(250 * 1024 * 1024)
	c.MultiThreadStreams = 4

	c.TrackRenamesStrategy = "hash"

	return c
}

type configContextKeyType struct{}

// Context key for config
var configContextKey = configContextKeyType{}

// GetConfig returns the global or context sensitive context
func GetConfig(ctx context.Context) *ConfigInfo {
	if ctx == nil {
		return globalConfig
	}
	c := ctx.Value(configContextKey)
	if c == nil {
		return globalConfig
	}
	return c.(*ConfigInfo)
}

// AddConfig returns a mutable config structure based on a shallow
// copy of that found in ctx and returns a new context with that added
// to it.
func AddConfig(ctx context.Context) (context.Context, *ConfigInfo) {
	c := GetConfig(ctx)
	cCopy := new(ConfigInfo)
	*cCopy = *c
	newCtx := context.WithValue(ctx, configContextKey, cCopy)
	return newCtx, cCopy
}

// ConfigToEnv converts a config section and name, e.g. ("myremote",
// "ignore-size") into an environment name
// "RCLONE_CONFIG_MYREMOTE_IGNORE_SIZE"
func ConfigToEnv(section, name string) string {
	return "RCLONE_CONFIG_" + strings.ToUpper(strings.Replace(section+"_"+name, "-", "_", -1))
}

// OptionToEnv converts an option name, e.g. "ignore-size" into an
// environment name "RCLONE_IGNORE_SIZE"
func OptionToEnv(name string) string {
	return "RCLONE_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
}
