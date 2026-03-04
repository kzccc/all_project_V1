package main
import (
    "fmt"
    "time"
)
func main() {
    ch := make(chan string)
    go sender(ch)         // sender goroutine
    go recver(ch)         // recver goroutine
    time.Sleep(1e9)
}
func sender(ch chan string) {
    ch <- "malongshuai"
    ch <- "gaoxiaofang"
    ch <- "wugui"
    ch <- "tuner"
}
func recver(ch chan string) {
    var recv string
    for {
        recv = <-ch
        fmt.Println(recv)
    }
}