// Copyright 2015 The go-zenanet Authors
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

package flags

import (
	"os/user"
	"runtime"
	"testing"
)

// nolint:paralleltest
func TestPathExpansion(t *testing.T) {
	user, _ := user.Current()

	var tests map[string]string

	if runtime.GOOS == "windows" {
		tests = map[string]string{
			`/home/someuser/tmp`:        `\home\someuser\tmp`,
			`~/tmp`:                     user.HomeDir + `\tmp`,
			`~thisOtherUser/b/`:         `~thisOtherUser\b`,
			`$DDDXXX/a/b`:               `\tmp\a\b`,
			`/a/b/`:                     `\a\b`,
			`C:\Documents\Newsletters\`: `C:\Documents\Newsletters`,
			`C:\`:                       `C:\`,
			`\\.\pipe\\pipe\gzen621383`: `\\.\pipe\\pipe\gzen621383`,
		}
	} else {
		tests = map[string]string{
			`/home/someuser/tmp`:        `/home/someuser/tmp`,
			`~/tmp`:                     user.HomeDir + `/tmp`,
			`~thisOtherUser/b/`:         `~thisOtherUser/b`,
			`$DDDXXX/a/b`:               `/tmp/a/b`,
			`/a/b/`:                     `/a/b`,
			`C:\Documents\Newsletters\`: `C:\Documents\Newsletters\`,
			`C:\`:                       `C:\`,
			`\\.\pipe\\pipe\gzen621383`: `\\.\pipe\\pipe\gzen621383`,
		}
	}

	t.Setenv(`DDDXXX`, `/tmp`)

	for test, expected := range tests {
		got := expandPath(test)
		if got != expected {
			t.Errorf(`test %s, got %s, expected %s\n`, test, got, expected)
		}
	}
}
