package constants

const (
	PIDFileName        = "pid"
	ConfigFileName     = "config.json"
	BundlePathFileName = "bundle.path"

	// default paths and values
	DefaultBundlePath = "."
	DefaultBasePath   = "./run/ror"

	// file permissions
	DefaultFilePermissions = 0644
	DefaultDirPermissions  = 0755

	// process signals and time outs (seconds)
	GracefulTerminationTimeout = 5
	ContainerStartTimeout      = 30

	// logging prefixes
	LogPrefixParent = "[PARENT]"
	LogPrefixChild  = "[CHILD]"
	LogPrefixWarn   = "[WARN]"
	LogPrefixInfo   = "[INFO]"

	ContainerBinPath = "/bin:/usr/bin:/sbin:/usr/sbin"

	// error msgs
	ErrContainerIDRequired = "container ID is required"
	ErrContainerNotFound   = "container does not exist"
	ErrInvalidContainerID  = "invalid container ID format"
)
