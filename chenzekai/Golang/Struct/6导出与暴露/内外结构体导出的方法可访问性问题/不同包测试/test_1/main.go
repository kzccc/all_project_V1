package main

import "example.com/struct/innerouter/abc"

func main() {
    a := new(abc.Admin)

    // 下面报错
    // a.speak()

    // 下面正常
    a.Sing()
}
