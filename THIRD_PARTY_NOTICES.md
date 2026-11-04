# Third-Party Software Notices

Nomos (`CGO_ENABLED=0` static Go distributions across Linux, macOS, and Windows) incorporates open-source Go modules governed by permissive licenses (**Apache-2.0**, **BSD-3-Clause**, **MIT**, and **Public Domain**). Zero copyleft (`GPL`, `LGPL`, `AGPL`, or `MPL`) components are linked into the `nomos` binary.

---

## 1. Apache License 2.0 Components

The full text of the Apache License, Version 2.0 is provided in [`LICENSE`](LICENSE).

- **`github.com/spf13/cobra`** (`v1.10.2`) — Apache-2.0  
  Copyright 2013-2023 Steve Francia and The Cobra Authors.
- **`github.com/inconshreveable/mousetrap`** (`v1.1.0`) — Apache-2.0  
  Copyright 2014 Alan Shreve.
- **`github.com/tetratelabs/wazero`** (`v1.12.0`) — Apache-2.0  
  `NOTICE`:
  ```text
  wazero
  Copyright 2020-2023 wazero authors
  ```
- **`gopkg.in/yaml.v3`** (`v3.0.1`) — Apache-2.0 and MIT  
  `NOTICE` (Apache-2.0 portion):
  ```text
  Copyright 2011-2019 Canonical Ltd.
  Licensed under the Apache License, Version 2.0 (the "License").
  ```
  MIT portion (`libyaml` Go port): Copyright (c) 2006-2011 Kirill Simonov (see MIT License below).

---

## 2. BSD 3-Clause & Public Domain Components

- **`golang.org/x/sys`** (`v0.49.0`) — BSD-3-Clause  
  Copyright (c) 2009 The Go Authors. All rights reserved.
- **`github.com/google/uuid`** (`v1.6.0`) — BSD-3-Clause  
  Copyright (c) 2009, 2014 Google Inc. All rights reserved.
- **`github.com/spf13/pflag`** (`v1.0.10`) — BSD-3-Clause  
  Copyright (c) 2012 Alex Ogier. All rights reserved.  
  Copyright (c) 2012 The Go Authors. All rights reserved.
- **`github.com/remyoudompheng/bigfft`** (`v0.0.0-20230129092748-24d4a6f8daec`) — BSD-3-Clause  
  Copyright (c) 2012 The Go Authors. All rights reserved.
- **`modernc.org/sqlite`** (`v1.61.0`) — BSD-3-Clause (with Public Domain SQLite C source translation and MIT `sqlite-vec` extension)  
  Copyright (c) 2017 The Sqlite Authors. All rights reserved.  
  SQLite core engine: Dedicated to the Public Domain by the SQLite authors.  
  Embedded `sqlite-vec` portion: Copyright (c) 2024 Alex Garcia (MIT License).
- **`modernc.org/libc`** (`v1.79.0`) — BSD-3-Clause  
  Copyright (c) 2020 The Libc Authors. All rights reserved.
- **`modernc.org/mathutil`** (`v1.7.1`) — BSD-3-Clause  
  Copyright (c) 2014 The mathutil Authors. All rights reserved.
- **`modernc.org/memory`** (`v1.12.1`) — BSD-3-Clause  
  Copyright (c) 2017 The Memory Authors. All rights reserved.  
  Copyright (c) 2009 The Go Authors. All rights reserved.  
  Copyright (c) 2011 Evan Shaw (`mmap-go`). All rights reserved.

### Standard BSD 3-Clause License Text

```text
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

---

## 3. MIT License Components

- **`github.com/charmbracelet/lipgloss`** (`v1.1.0`) — Copyright (c) 2021-2023 Charmbracelet, Inc.
- **`github.com/charmbracelet/colorprofile`** (`v0.2.3-0.20250311203215-f60798e515dc`) — Copyright (c) 2020-2024 Charmbracelet, Inc.
- **`github.com/charmbracelet/x/ansi`** (`v0.8.0`) — Copyright (c) 2023 Charmbracelet, Inc.
- **`github.com/charmbracelet/x/cellbuf`** (`v0.0.13-0.20250311204145-2c3ea96c31dd`) — Copyright (c) 2023 Charmbracelet, Inc.
- **`github.com/charmbracelet/x/term`** (`v0.2.1`) — Copyright (c) 2023 Charmbracelet, Inc.
- **`github.com/muesli/termenv`** (`v0.16.0`) — Copyright (c) 2019 Christian Muehlhaeuser.
- **`github.com/aymanbagabas/go-osc52/v2`** (`v2.0.1`) — Copyright (c) 2022 Ayman Bagabas.
- **`github.com/dustin/go-humanize`** (`v1.1.0`) — Copyright (c) 2005-2008 Dustin Sallings.
- **`github.com/lucasb-eyer/go-colorful`** (`v1.2.0`) — Copyright (c) 2013 Lucas Beyer.
- **`github.com/mattn/go-isatty`** (`v0.0.24`) — Copyright (c) Yasuhiro Matsumoto.
- **`github.com/mattn/go-runewidth`** (`v0.0.16`) — Copyright (c) 2016 Yasuhiro Matsumoto.
- **`github.com/ncruces/go-strftime`** (`v1.1.0`) — Copyright (c) 2022 Nuno Cruces.
- **`github.com/rivo/uniseg`** (`v0.4.7`) — Copyright (c) 2019 Oliver Kuederle.
- **`github.com/xo/terminfo`** (`v0.0.0-20220910002029-abceb7e1c41e`) — Copyright (c) 2016 Anmol Sethi.
- **`gopkg.in/yaml.v3` (`libyaml` Go port)** (`v3.0.1`) — Copyright (c) 2006-2011 Kirill Simonov.

### Standard MIT License Text

```text
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
