/**
 * @license
 * Copyright 2023 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

interface Greeter {
  (target: string): void;
}

const hello: Greeter = (target) => console.log(`Hello, ${target}!`);

hello("world");

export {};
