//
// command/distribute.go
//
// Copyright (c) 2017 Junpei Kawamoto
//
// This file is part of sss.
//
// sss is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// sss is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with sss.  If not, see <http://www.gnu.org/licenses/>.
//

package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/itslab-kyushu/sss/sss"
	"github.com/ulikunitz/xz"
	"github.com/urfave/cli"
)

type distributeOpt struct {
	Filename  string
	ChunkSize int
	Size      int
	Threshold int
}

// CmdDistribute executes distribute command.
func CmdDistribute(c *cli.Context) (err error) {

	if c.NArg() != 3 {
		return cli.ShowSubcommandHelp(c)
	}

	threshold, err := strconv.Atoi(c.Args().Get(1))
	if err != nil {
		return
	}
	size, err := strconv.Atoi(c.Args().Get(2))
	if err != nil {
		return
	}

	return cmdDistribute(&distributeOpt{
		Filename:  c.Args().Get(0),
		ChunkSize: c.Int("chunk"),
		Size:      size,
		Threshold: threshold,
	})
}

func cmdDistribute(opt *distributeOpt) (err error) {

	secret, err := ioutil.ReadFile(opt.Filename)
	if err != nil {
		return
	}

	shares, err := sss.Distribute(secret, opt.ChunkSize, opt.Size, opt.Threshold)
	if err != nil {
		return
	}

	wg, ctx := errgroup.WithContext(context.Background())
	semaphore := make(chan struct{}, runtime.NumCPU())
	for i, s := range shares {

		func(i int, s sss.Share) {

			select {
			case <-ctx.Done():
				return
			case semaphore <- struct{}{}:
				wg.Go(func() (err error) {
					defer func() { <-semaphore }()

					data, err := json.Marshal(s)
					if err != nil {
						return
					}

					fp, err := os.OpenFile(fmt.Sprintf("%s.%d.xz", opt.Filename, i), os.O_WRONLY|os.O_CREATE, 0644)
					if err != nil {
						return
					}
					defer fp.Close()

					w, err := xz.NewWriter(fp)
					if err != nil {
						return
					}
					defer w.Close()

					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
							n, err := w.Write(data)
							if err != nil {
								return err
							}
							if n == len(data) {
								return nil
							}
							data = data[n:]
						}
					}

				})
			}

		}(i, s)

	}

	return wg.Wait()
}
