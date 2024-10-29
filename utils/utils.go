package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danthegoodman1/raftd/gologger"
	"github.com/labstack/echo/v4"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/segmentio/ksuid"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

var logger = gologger.NewLogger()

func GetEnvOrDefault(env, defaultVal string) string {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		return e
	}
}

func GetEnvOrDefaultInt(env string, defaultVal int64) int64 {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		intVal, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			logger.Error().Msg(fmt.Sprintf("Failed to parse string to int '%s'", env))
			os.Exit(1)
		}

		return intVal
	}
}

func MustEnv(env string) string {
	e := os.Getenv(env)
	if e == "" {
		logger.Fatal().Msgf("Must provide env var %s", env)
		return ""
	}

	return e
}

func GenRandomID(prefix string) string {
	return prefix + gonanoid.MustGenerate("abcdefghijklmonpqrstuvwxyzABCDEFGHIJKLMONPQRSTUVWXYZ0123456789", 22)
}

func GenKSortedID(prefix string) string {
	return prefix + ksuid.New().String()
}

func GenRandomShortID() string {
	// reduced character set that's less probable to mis-type
	// change for conflicts is still only 1:128 trillion
	return gonanoid.MustGenerate("abcdefghikmonpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ0123456789", 8)
}

type NoEscapeJSONSerializer struct{}

var _ echo.JSONSerializer = &NoEscapeJSONSerializer{}

func (d *NoEscapeJSONSerializer) Serialize(c echo.Context, i interface{}, indent string) error {
	enc := json.NewEncoder(c.Response())
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

// Deserialize reads a JSON from a request body and converts it into an interface.
func (d *NoEscapeJSONSerializer) Deserialize(c echo.Context, i interface{}) error {
	// Does not escape <, >, and ?
	err := json.NewDecoder(c.Request().Body).Decode(i)
	var ute *json.UnmarshalTypeError
	var se *json.SyntaxError
	if ok := errors.As(err, &ute); ok {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v", ute.Type, ute.Value, ute.Field, ute.Offset)).SetInternal(err)
	} else if ok := errors.As(err, &se); ok {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Syntax error: offset=%v, error=%v", se.Offset, se.Error())).SetInternal(err)
	}
	return err
}

func IfElse[T any](check bool, a T, b T) T {
	if check {
		return a
	}
	return b
}

func OrEmptyArray[T any](a []T) []T {
	if a == nil {
		return make([]T, 0)
	}
	return a
}

func FirstOr[T any](a []T, def T) T {
	if len(a) == 0 {
		return def
	}
	return a[0]
}

func Ptr[T any](val T) *T {
	return &val
}

var ErrVersionBadFormat = PermError("bad version format")

// VersionToInt converts a simple semantic version string (e.e. 18.02.66)
func VersionToInt(v string) (int64, error) {
	sParts := strings.Split(v, ".")
	if len(sParts) > 3 {
		return -1, ErrVersionBadFormat
	}
	var iParts = make([]int64, 3)
	for i := range sParts {
		vp, err := strconv.ParseInt(sParts[i], 10, 64)
		if err != nil {
			return -1, fmt.Errorf("error in ParseInt: %s %w", err.Error(), ErrVersionBadFormat)
		}
		iParts[i] = vp
	}
	return iParts[0]*10_000*10_000 + iParts[1]*10_000 + iParts[2], nil
}

// FuncNameFQ returns the fully qualified name of the function.
func FuncNameFQ(f any) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// FuncName returns the name of the function, without the package.
func FuncName(f any) string {
	fqName := FuncNameFQ(f)
	return fqName[strings.LastIndexByte(fqName, '.')+1:]
}

func AsErr[T error](err error) (te T, ok bool) {
	if err == nil {
		return te, false
	}
	return te, errors.As(err, &te)
}

// IsErr is useful for check for a class of errors (e.g. *serviceerror.WorkflowExecutionAlreadyStarted) instead of a specific error.
// E.g. Temporal doesn't even expose some errors, only their types
func IsErr[T error](err error) bool {
	_, ok := AsErr[T](err)
	return ok
}

func MustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// WriteFileAtomic atomically writes to a file, even if it already exists
// from https://github.com/tailscale/tailscale/blob/main/atomicfile/atomicfile.go
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) (err error) {
	fi, err := os.Stat(filename)
	if err == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("%s already exists and is not a regular file", filename)
	}
	f, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmpName)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := f.Chmod(perm); err != nil {
			return err
		}
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filename)
}
