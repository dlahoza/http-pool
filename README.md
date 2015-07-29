# http-pool
Extension of net/http package from standard Go library with high performance pool manager
## Setup
```
$ sudo wget -O/usr/local/go/src/net/http/pool.go https://raw.githubusercontent.com/DLag/http-pool/master/pool.go
$ cd /usr/local/go/src; sudo ./all.bash
```
##Example
```
package main

import (
	"fmt"
	"net/http"
	"log"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Workers: %d/%d!", p.GetBusy(), p.GetTotal())
}

var p *http.Pool

func main() {
	http.HandleFunc("/", handler)
	p = http.NewPool(1000)
	err := http.ListenAndServeWithPool(":8080", nil, p)
	if err != nil {
		log.Fatal("ListenAndServeWithPool: ", err)
	}
}
```
