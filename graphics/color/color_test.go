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

package color

// The following types implement the Color interface.
var (
	_ Color = colorDeviceGray(0)
	_ Color = colorDeviceRGB{0, 0, 0}
	_ Color = colorDeviceCMYK{0, 0, 0, 1}
	_ Color = colorCalGray{}
	_ Color = colorCalRGB{}
	_ Color = colorLab{}
	_ Color = colorICCBased{}
	_ Color = colorColoredPattern{}
	_ Color = (*colorUncoloredPattern)(nil)
	_ Color = colorIndexed{}
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)
