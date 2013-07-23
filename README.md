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

##Siege vs gobench:

###Siege:

    $>siege -b -t10S -c500 http://localhost:80/
    
    ** SIEGE 2.70
    ** Preparing 500 concurrent users for battle.
    The server is now under siege...
    Lifting the server siege...      done.
    Transactions:		       72342 hits
    Availability:		      100.00 %
    Elapsed time:		        9.29 secs
    Data transferred:	       88.24 MB
    Response time:		        0.06 secs
    Transaction rate:	     7787.08 trans/sec
    Throughput:		        9.50 MB/sec
    Concurrency:		      490.07
    Successful transactions:           0
    Failed transactions:	           0
    Longest transaction:	        0.84
    Shortest transaction:	        0.00
    
###gobench:

    $>go run gobench.go -u http://localhost:80 -c 500 -t 10

    Dispatching 500 clients
    Waiting for results...

    Requests:                249333 hits
    Successful requests:     249333 hits
    Network failed:               0 hits
    Bad failed:                   0 hits
    Requests rate:            24715 hits/sec

* requests hits and requests rate are 3X better on the same time (10 seconds) and the same number of clients (500)!
* I try the same with 2000 clients on Siege with proper system configuration, and Siege was crashed
* I try gobench with the maximum number of clients that we can use (65535 ports) - it's rocked!
* I didn't put yet the results of ab because I still need to investigate the results


Usage
================

1. run some http server on port 80
2. go run gobench.go -u http://localhost:80 -k=true -c 500 -t 10


Help
================

go run gobench.go --help


License
================

Licensed under the New BSD License.


Author
================

Uri Shamay (shamayuri@gmail.com)
