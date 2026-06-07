package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/model"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
)

const defaultEnvFile = ".env"

type LoadOptions struct {
	EnvFile string
}

type Options struct {
	StorageProvider string
	StoragePrefix   string
	StorageBaseURL  string
	LocalRoot       string
	Bucket          string
	Region          string
	Endpoint        string

	S3ForcePathStyle bool
	OSSUsePathStyle  bool
	OSSUseCName      bool

	DefaultMaxSize        int64
	DefaultTempTTLSeconds int
	DefaultAllowedExts    string
	DefaultAllowedMimes   string
}

func Load(opts LoadOptions) (Options, error) {
	envFile := strings.TrimSpace(opts.EnvFile)
	if envFile == "" {
		envFile = strings.TrimSpace(os.Getenv("FBA_ENV_FILE"))
	}
	if envFile == "" {
		envFile = defaultEnvFile
	}
	values, err := readDotEnv(envFile)
	if err != nil {
		return Options{}, err
	}
	mergeProcessEnv(values)
	return optionsFromValues(values)
}

func ApplyToSeed(seed repo.Seed, opts Options) (repo.Seed, error) {
	provider := strings.TrimSpace(opts.StorageProvider)
	if provider == "" {
		provider = model.ProviderLocal
	}
	if provider != model.ProviderLocal && provider != model.ProviderS3 && provider != model.ProviderOSS {
		return repo.Seed{}, fmt.Errorf("unsupported uploadfile storage provider %q", provider)
	}

	seed.Storages = append([]model.Storage(nil), seed.Storages...)
	seed.Scenes = append([]model.Scene(nil), seed.Scenes...)
	applyStorageOptions(&seed, provider, opts)
	if err := applyDefaultSceneOptions(&seed, opts); err != nil {
		return repo.Seed{}, err
	}
	return seed, nil
}

func applyStorageOptions(seed *repo.Seed, provider string, opts Options) {
	if len(seed.Storages) == 0 {
		seed.Storages = model.SeedStorages()
	}
	index := 0
	for i, item := range seed.Storages {
		if item.Code == model.DefaultStorageCode {
			index = i
			break
		}
	}

	storage := seed.Storages[index]
	storage.Provider = provider
	if strings.TrimSpace(opts.StoragePrefix) != "" {
		storage.Prefix = strings.TrimSpace(opts.StoragePrefix)
	}
	storage.BaseURL = optionalString(opts.StorageBaseURL)
	storage.Bucket = optionalString(opts.Bucket)
	storage.Region = optionalString(opts.Region)
	storage.Endpoint = optionalString(opts.Endpoint)
	storage.IsDefault = true
	storage.Enabled = true
	storage.Config = storageConfig(provider, opts)
	seed.Storages[index] = storage
}

func applyDefaultSceneOptions(seed *repo.Seed, opts Options) error {
	for i := range seed.Scenes {
		if seed.Scenes[i].Code != model.DefaultSceneCode {
			continue
		}
		if opts.DefaultMaxSize > 0 {
			seed.Scenes[i].MaxSize = opts.DefaultMaxSize
		}
		if opts.DefaultTempTTLSeconds > 0 {
			seed.Scenes[i].TempTTLSeconds = opts.DefaultTempTTLSeconds
		}
		if strings.TrimSpace(opts.DefaultAllowedExts) != "" {
			normalized, err := normalizeStringList(opts.DefaultAllowedExts)
			if err != nil {
				return fmt.Errorf("UPLOADFILE_DEFAULT_ALLOWED_EXTS: %w", err)
			}
			seed.Scenes[i].AllowedExts = &normalized
		}
		if strings.TrimSpace(opts.DefaultAllowedMimes) != "" {
			normalized, err := normalizeStringList(opts.DefaultAllowedMimes)
			if err != nil {
				return fmt.Errorf("UPLOADFILE_DEFAULT_ALLOWED_MIMES: %w", err)
			}
			seed.Scenes[i].AllowedMimes = &normalized
		}
		return nil
	}
	return nil
}

func storageConfig(provider string, opts Options) *string {
	switch provider {
	case model.ProviderLocal:
		cfg := localStorageConfig{
			Root:    strings.TrimSpace(opts.LocalRoot),
			BaseURL: strings.TrimSpace(opts.StorageBaseURL),
		}
		if cfg.Root == "" && cfg.BaseURL == "" {
			return nil
		}
		return jsonString(cfg)
	case model.ProviderS3:
		if !opts.S3ForcePathStyle {
			return nil
		}
		return jsonString(s3StorageConfig{ForcePathStyle: true})
	case model.ProviderOSS:
		if !opts.OSSUsePathStyle && !opts.OSSUseCName {
			return nil
		}
		return jsonString(ossStorageConfig{UsePathStyle: opts.OSSUsePathStyle, UseCName: opts.OSSUseCName})
	default:
		return nil
	}
}

type localStorageConfig struct {
	Root    string `json:"root,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

type s3StorageConfig struct {
	ForcePathStyle bool `json:"force_path_style"`
}

type ossStorageConfig struct {
	UsePathStyle bool `json:"use_path_style"`
	UseCName     bool `json:"use_cname"`
}

func jsonString(value any) *string {
	raw, _ := json.Marshal(value)
	text := string(raw)
	return &text
}

func optionsFromValues(values map[string]string) (Options, error) {
	opts := Options{
		StorageProvider:     strings.TrimSpace(values["UPLOADFILE_STORAGE_PROVIDER"]),
		StoragePrefix:       strings.TrimSpace(values["UPLOADFILE_STORAGE_PREFIX"]),
		StorageBaseURL:      strings.TrimSpace(values["UPLOADFILE_STORAGE_BASE_URL"]),
		LocalRoot:           strings.TrimSpace(values["UPLOADFILE_LOCAL_ROOT"]),
		Bucket:              strings.TrimSpace(values["UPLOADFILE_STORAGE_BUCKET"]),
		Region:              strings.TrimSpace(values["UPLOADFILE_STORAGE_REGION"]),
		Endpoint:            strings.TrimSpace(values["UPLOADFILE_STORAGE_ENDPOINT"]),
		DefaultAllowedExts:  strings.TrimSpace(values["UPLOADFILE_DEFAULT_ALLOWED_EXTS"]),
		DefaultAllowedMimes: strings.TrimSpace(values["UPLOADFILE_DEFAULT_ALLOWED_MIMES"]),
	}
	var err error
	if opts.S3ForcePathStyle, err = boolValue(values, "UPLOADFILE_S3_FORCE_PATH_STYLE"); err != nil {
		return Options{}, err
	}
	if opts.OSSUsePathStyle, err = boolValue(values, "UPLOADFILE_OSS_USE_PATH_STYLE"); err != nil {
		return Options{}, err
	}
	if opts.OSSUseCName, err = boolValue(values, "UPLOADFILE_OSS_USE_CNAME"); err != nil {
		return Options{}, err
	}
	if opts.DefaultMaxSize, err = int64Value(values, "UPLOADFILE_DEFAULT_MAX_SIZE"); err != nil {
		return Options{}, err
	}
	ttl, err := intValue(values, "UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS")
	if err != nil {
		return Options{}, err
	}
	opts.DefaultTempTTLSeconds = ttl
	return opts, nil
}

func readDotEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return values, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parse uploadfile dotenv %s:%d: missing '='", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("parse uploadfile dotenv %s:%d: empty key", path, lineNo)
		}
		values[key] = parseDotEnvValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func parseDotEnvValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			value = value[1 : len(value)-1]
			if quote == '"' {
				value = strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t", `\"`, `"`, `\\`, `\`).Replace(value)
			}
			return value
		}
	}
	if index := strings.Index(value, " #"); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func mergeProcessEnv(values map[string]string) {
	for _, key := range uploadfileEnvKeys {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			values[key] = value
		}
	}
}

var uploadfileEnvKeys = []string{
	"UPLOADFILE_STORAGE_PROVIDER",
	"UPLOADFILE_STORAGE_PREFIX",
	"UPLOADFILE_STORAGE_BASE_URL",
	"UPLOADFILE_LOCAL_ROOT",
	"UPLOADFILE_STORAGE_BUCKET",
	"UPLOADFILE_STORAGE_REGION",
	"UPLOADFILE_STORAGE_ENDPOINT",
	"UPLOADFILE_S3_FORCE_PATH_STYLE",
	"UPLOADFILE_OSS_USE_PATH_STYLE",
	"UPLOADFILE_OSS_USE_CNAME",
	"UPLOADFILE_DEFAULT_MAX_SIZE",
	"UPLOADFILE_DEFAULT_TEMP_TTL_SECONDS",
	"UPLOADFILE_DEFAULT_ALLOWED_EXTS",
	"UPLOADFILE_DEFAULT_ALLOWED_MIMES",
}

func boolValue(values map[string]string, key string) (bool, error) {
	raw := strings.TrimSpace(values[key])
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return value, nil
}

func int64Value(values map[string]string, key string) (int64, error) {
	raw := strings.TrimSpace(values[key])
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return value, nil
}

func intValue(values map[string]string, key string) (int, error) {
	value, err := int64Value(values, key)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func normalizeStringList(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	var values []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return "", err
		}
	} else {
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(strings.TrimPrefix(item, "."))
			if item != "" {
				values = append(values, item)
			}
		}
	}
	clean := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(strings.TrimPrefix(item, "."))
		if item != "" {
			clean = append(clean, item)
		}
	}
	rawJSON, err := json.Marshal(clean)
	if err != nil {
		return "", err
	}
	return string(rawJSON), nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
