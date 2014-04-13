Introduction
================

I wrote that code because: (the obvious reason::I love to write code in Go)

We are working so hard to optimize our servers - why shouldn't we do it for our clients testers?!

I noticed that the existing tools for benchmarking/load HTTP/HTTPS servers has some issues:
* ab (ApacheBenchmark) - maximum concurrency is 20k (can be eliminate by opening multiple ab processes)
* Siege are work in a model of native thread per request, meaning that you cannot simulated thousands/ten of thousands clients concurrently even if you tweak the RLIMIT of stack usage per native thread - still that kill the client machine and cause it to be very load and not an efficient client/s.
What we really want is minimum resources usage and get the maximum throughput/load!

If you already familiar with the model of Go for high performance I/O and goroutines we can achieve that mission easily.

The funny part - I do some benchmark to the client tester tool and not to the server:

##Siege vs GoBench:

###Siege:

    $>siege -b -t10S -c500 http://localhost:80/
    
    ** SIEGE 2.70
    ** Preparing 500 concurrent users for battle.
    The server is now under siege...
    Lifting the server siege...      done.
    Transactions:		       74247 hits
    Availability:		      100.00 %
    Elapsed time:		        9.62 secs
    Data transferred:	       96.58 MB
    Response time:		        0.06 secs
    Transaction rate:	     7717.98 trans/sec
    Throughput:		       10.04 MB/sec
    Concurrency:		      490.19
    Successful transactions:       74247
    Failed transactions:	           0
    Longest transaction:	        1.02
    Shortest transaction:	        0.00
    
###GoBench:

    $>go run gobench.go -k=true -u http://localhost:80 -c 500 -t 10
    Dispatching 500 clients
    Waiting for results...

    Requests:                           343669 hits
    Successful requests:                343669 hits
    Network failed:                          0 hits
    Bad requests failed (!2xx):              0 hits
    Successfull requests rate:           34366 hits/sec
    Read throughput:                  54700061 bytes/sec
    Write throughput:                  4128684 bytes/sec
    Test time:                              10 sec


* requests hits and requests rate are 5X better on the same time (10 seconds) and the same number of clients (500)!
* I try the same with 2000 clients on Siege with proper system configuration, and Siege was crashed
* I try gobench with the maximum number of clients that we can use (65535 ports) - it's rocked!
* I didn't put yet the results of ab because I still need to investigate the results

Usage
================

1. run some http server on port 80
2. run gobench for HTTP GET

    ```$>go run gobench.go -u http://localhost:80 -k=true -c 500 -t 10```
    
3. run gobench for HTTP POST

    ```$>go run gobench.go -u http://localhost:80 -k=true -c 500 -t 10 -d /tmp/post```


Notes
================

1. build a binary: 

    ```$>go build gobench.go```
    
2. Because it's a test tool, in HTTPS the ceritificate verification is insecure
3. use Go >= 1.1 (1.1 including major bug fixes)

Help
================

```go run gobench.go --help```

License
================

Licensed under the New BSD License.

Author
================

Uri Shamay (shamayuri@gmail.com)
