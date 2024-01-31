// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package graphics

// A Recorder allows to record graphics commands.
// The recorded commands can then later be applied to a [Writer],
// using the [Recorder.ApplyTo] method.
type Recorder struct {
	cmds []*recordedCmd
}

type recordedCmd struct {
	op   op
	args []any
}

type op int

const (
	opMoveTo op = iota
	opLineTo
	opCurveTo
	opClosePath
	opStroke
	opFill
	opFillAndStroke
	opClip
	opSetLineWidth
	opSetLineCap
	opSetLineJoin
	opSetMiterLimit
)

// ApplyTo applies all recorded commands to the given writer.
func (r *Recorder) ApplyTo(w *Writer) {
	for _, cmd := range r.cmds {
		switch cmd.op {
		case opSetLineWidth:
			w.SetLineWidth(cmd.args[0].(float64))
		}
	}
}

// SetLineWidth sets the line width.
//
// This implementes the PDF graphics operator "w".
func (r *Recorder) SetLineWidth(width float64) {
	r.cmds = append(r.cmds, &recordedCmd{opSetLineWidth, []any{width}})
}
