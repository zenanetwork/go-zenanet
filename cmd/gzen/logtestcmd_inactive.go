//go:build !integrationtests

// Copyright 2023 The go-zenanet Authors
// This file is part of go-zenanet.
//
// go-zenanet is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-zenanet is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-zenanet. If not, see <http://www.gnu.org/licenses/>.

package main

import "github.com/urfave/cli/v2"

var logTestCommand *cli.Command
