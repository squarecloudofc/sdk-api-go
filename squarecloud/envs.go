package squarecloud

// EnvVars is the application environment variables as a key → value map.
// Up to 256 entries per application; keys max 1024 chars, values max 4096.
type EnvVars map[string]string
