// Copyright 2023 The go-zenanet Authors
// This file is part of the go-zenanet library.
//
// The go-zenanet library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-zenanet library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-zenanet library. If not, see <http://www.gnu.org/licenses/>.

package forks

// Fork is a numerical identifier of specific network upgrades (forks).
type Fork int

const (
	Frontier = iota
	FrontierThawing
	Homestead
	DAO
	TangerineWhistle
	SpuriousDragon
	Byzantium
	Constantinople
	Petersburg
	Istanbul
	MuirGlacier
	Berlin
	London
	ArrowGlacier
	GrayGlacier
	Paris
	Shanghai
	Cancun
	Prague
)
