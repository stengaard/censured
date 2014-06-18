censured
========

A simple tool to try and figure out if some countries are blocking
your website.

How does it work?
=================

You give it the known content of a URL and list of open proxies.
It simply connects, to the known URL through each of the proxies
and compares the result to the known value. If they differ we
assume it is a block.


FAQ
===

 * How do I install it?

    go get github.com/stengaard/censured


  * Where do I get a list of proxies?
    http://proxyfire.net/
    http://getfoxyproxy.org/proxylists.html
    http://www.xroxy.com/proxylist.htm
    http://proxylist.hidemyass.com/
