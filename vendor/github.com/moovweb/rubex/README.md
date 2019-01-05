# Rubex : Super Fast Regexp for Go #
by Zhigang Chen (zhigang.chen@moovweb.com or zhigangc@gmail.com)

***ONLY USE go1 BRANCH***

A simple regular expression library that supports Ruby's regexp syntax. It implements all the public functions of Go's Regexp package, except LiteralPrefix. By the benchmark tests in Regexp, the library is 40% to 10X faster than Regexp on all but one test. Unlike Go's Regrexp, this library supports named capture groups and also allow "\\1" and "\\k<name>" in replacement strings.

The library calls the Oniguruma regex library (5.9.2, the latest release as of now) for regex pattern searching. All replacement code is done in Go. This library can be easily adapted to support the regex syntax used by other programming languages or tools, like Java, Perl, grep, and emacs.

## Installation ##

First, ensure you have Oniguruma installed. On OS X with brew, its as simple as
    
    brew install oniguruma
    
On Ubuntu...

    sudo apt-get install libonig2

Now that we've got Oniguruma installed, we can install Rubex!

    go install github.com/moovweb/rubex

## Example Usage ##

    import "rubex"
    
    rxp := rubex.MustCompile("[a-z]*")
    if err != nil {
        // whoops
    }
    result := rxp.FindString("a me my")
    if result != "" {
        // FOUND A STRING!! YAY! Must be "a" in this instance
    } else {
        // no good
    }

