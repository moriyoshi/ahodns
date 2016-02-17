AhoDNS
======

AhoDNS is a tiny and simple forward-lookup only DNS server written in Go using https://github.com/miekg/dns .

```
$ ./ahodns -listen="udp:127.0.0.1:8053" -ttl=300 RECORDFILE
```

By default it tries to listen on 127.0.0.1:8053.  To test with this configuration, do the following:

```
$ dig @127.0.0.1 -p 8053 aho
```

Record file format
------------------

It is a simple text file that contains a pair of a domain name and an IP address in each line.  Beware that the values are separated by a tab, not spaces!

```
aho1	127.0.0.2
aho1	127.0.0.3
aho2	127.0.0.4
aho2	::ffff:127.0.0.5
```
