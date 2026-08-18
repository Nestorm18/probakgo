package version

// Version is the single release version shared by all probakgo binaries.
// Release builds override it with -ldflags "-X probakgo/internal/version.Version=...".
var Version = "0.0.215"
