# Traceroute in Go

A traceroute library written in Go.

Features:
  * blocking and non blocking mode
  * structured output, in text or JSON
  * configurable options like: resolve domain names, startTTL, payloadSize, timeouts, retries
  * works correctly when launching in multiple concurrent processes
  * doesn't catch ICMP replies from someone's else processes 

To perform network operations, syscalls and RAW_SOCKETS are used. 
Therefore, in Linux, executing the command requires root privileges, 
or sudo, or you can set the SET_CAP_RAW flag on the executable file using the setcap command:
```setcap cap_net_raw+ep /path_to_exec_file```

This library uses BPF (Berkley packet filter) connected to the socket in order to filter network packets at the kernel side.
BPF isn't supported on Windows and is not tested on Mac. I have no test environment to check this cases. 
BPF can be disabled on Windows/Mac with the loss of the opportunity to work in a competitive mode.

Only 1024 concurrent 'traceroutes' at the same time is supported. 
More concurrent traceroutes is allowed, but it leads to some packets would be lost. 


## CLI App

```sh
go build cmd/gotraceroute
sudo ./gotraceroute example.com
```

## Library

See traceroute_test.go for an example of how to use the library from within your application.

The traceroute.Run() function accepts a domain name and an options struct and immediately returns with a channel where a Hop data struct should be reading from. When traceroute is finished, the channel will be closed.

The traceroute.RunBlock() function accepts a domain name and an options struct, perform a traceroute and return array of Hop structs with traceroute result.

## Resources

Useful resources:

* http://en.wikipedia.org/wiki/Traceroute
* http://tools.ietf.org/html/rfc792
* http://en.wikipedia.org/wiki/Internet_Control_Message_Protocol

## Notes

* https://code.google.com/p/go/source/browse/src/pkg/net/ipraw_test.go
* http://godoc.org/code.google.com/p/go.net/ipv4
* http://golang.org/pkg/syscall/


## Thanks

Based on traceroute implementation https://github.com/aeden/traceroute which was fully reworked and
as a result several annoying bugs was fixed, error handling was added, and it was adopted to concurrent execution.
Some ideas about packet construction and decoding also was get from https://github.com/Syncbak-Git/traceroute

