package chains

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/zenanetwork/go-zenanet/core/types"
)

//go:embed allocs
var allocs embed.FS

func readPrealloc(filename string) types.GenesisAlloc {
	f, err := allocs.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("Could not open genesis preallocation for %s: %v", filename, err))
	}

	defer f.Close()
	decoder := json.NewDecoder(f)
	ga := make(types.GenesisAlloc)

	err = decoder.Decode(&ga)
	if err != nil {
		panic(fmt.Sprintf("Could not parse genesis preallocation for %s: %v", filename, err))
	}

	return ga
}
