package chains

import (
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/params"
)

var eireneTestnet = &Chain{
	Hash:      common.HexToHash("0x7b66506a9ebdbf30d32b43c5f15a3b1216269a1ec3a75aa3182b86176a2b1ca7"),
	NetworkId: 80001,
	Genesis: &core.Genesis{
		Config: &params.ChainConfig{
			ChainID:             big.NewInt(80001),
			HomesteadBlock:      big.NewInt(0),
			DAOForkBlock:        nil,
			DAOForkSupport:      true,
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(2722000),
			MuirGlacierBlock:    big.NewInt(2722000),
			BerlinBlock:         big.NewInt(13996000),
			LondonBlock:         big.NewInt(22640000),
			ShanghaiBlock:       big.NewInt(41874000),
			CancunBlock:         big.NewInt(45648608),
			Zena: &params.ZenaConfig{
				Period: map[string]uint64{
					"0":        2,
					"25275000": 5,
					"29638656": 2,
				},
				ProducerDelay: map[string]uint64{
					"0":        6,
					"29638656": 4,
				},
				Sprint: map[string]uint64{
					"0":        64,
					"29638656": 16,
				},
				BackupMultiplier: map[string]uint64{
					"0":        2,
					"25275000": 5,
					"29638656": 2,
				},
				ValidatorContract: "0x0000000000000000000000000000000000001000",
			},
		},
		Nonce:      0,
		Timestamp:  1558348305,
		GasLimit:   10000000,
		Difficulty: big.NewInt(1),
		Mixhash:    common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
		Alloc:      readPrealloc("allocs/eirene.json"),
	},
	Bootnodes: []string{
		"enode://bdcd4786a616a853b8a041f53496d853c68d99d54ff305615cd91c03cd56895e0a7f6e9f35dbf89131044e2114a9a782b792b5661e3aff07faf125a98606a071@43.200.206.40:30303",
		"enode://209aaf7ed549cf4a5700fd833da25413f80a1248bd3aa7fe2a87203e3f7b236dd729579e5c8df61c97bf508281bae4969d6de76a7393bcbd04a0af70270333b3@54.216.248.9:30303",
	},
}
