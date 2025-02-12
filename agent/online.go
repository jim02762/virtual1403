package main

// Copyright 2021 Matthew R. Wilson <mwilson@mattwilson.org>
//
// This file is part of virtual1403
// <https://github.com/racingmars/virtual1403>.
//
// virtual1403 is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// virtual1403 is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with virtual1403. If not, see <https://www.gnu.org/licenses/>.

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/klauspost/compress/zstd"
	"github.com/racingmars/virtual1403/scanner"
)

type onlineOutputHandler struct {
	buf bytes.Buffer
	enc *zstd.Encoder
	w   *bufio.Writer
	api string
	key string
}

func newOnlineOutputHandler(api, key string) scanner.PrinterHandler {
	o := &onlineOutputHandler{
		api: api,
		key: key,
	}
	o.enc, _ = zstd.NewWriter(&o.buf)
	o.w = bufio.NewWriter(o.enc)

	return o
}

func (o *onlineOutputHandler) AddLine(line string, linefeed bool) {
	command := "L:"
	if !linefeed {
		command = "O:"
	}
	o.w.WriteString(command + line + "\n")
}

func (o *onlineOutputHandler) PageBreak() {
	o.w.WriteString("P:\n")
}

func (o *onlineOutputHandler) EndOfJob(jobinfo string) {
	o.w.WriteString("J:" + jobinfo + "\n")

	// No matter what happens, we always want to reset our state to a fresh
	// new job.
	defer func() {
		// We could use Buffer.Reset(), but if this was a particularly large
		// job, there's no reason for us to hold on to that much allocated
		// memory indefinitely. All things considered, this is a low-volume
		// application to paying for the allocation of a new buffer slice
		// isn't going to have a noticable performance penalty.
		o.buf = bytes.Buffer{}
		o.enc, _ = zstd.NewWriter(&o.buf)
		o.w = bufio.NewWriter(o.enc)
	}()

	o.w.Flush()
	o.enc.Close()

	// We now have a complete zstd-compressed job stream in o.buf.

	req, err := http.NewRequest(http.MethodPost,
		o.api, &o.buf)
	if err != nil {
		log.Printf("ERROR: unable to create HTTP request: %v", err)
		return
	}

	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("Content-Type", "text/x-print-job")
	req.Header.Set("Authorization", "Bearer "+o.key)

	log.Printf("INFO:  Sending print job to online print API...")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("ERROR: unable to execute HTTP request: %v", err)
		return
	}
	defer resp.Body.Close()
	defer io.ReadAll(resp.Body) // ensure keep-alive client reuse when able

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("INFO:  Print API response status: %s", resp.Status)
	} else {
		log.Printf("ERROR: Print API response status: %s", resp.Status)
	}
}
