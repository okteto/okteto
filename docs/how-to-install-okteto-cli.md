Okteto provides a local development experience for Kubernetes applications. You code locally in your favorite IDE and Okteto synchronizes it automatically to your cluster. The Okteto CLI is open source, and the code is available at [GitHub](https://github.com/okteto/okteto). It is a client-side only tool that works in any Kubernetes cluster.

Install the Okteto CLI following these steps:

### MacOS / Linux

```console
curl https://get.okteto.com -sSfL | sh
```

You can also install via [brew](https://brew.sh/) by running:

```console
brew install okteto
```

### Windows

Download [https://downloads.okteto.com/cli/okteto.exe](https://downloads.okteto.com/cli/okteto.exe) and add it to your `$PATH`.

You can also install via [scoop](https://scoop.sh/) by running:

```console
scoop install okteto
```

### GitHub

Alternatively, you can directly download the binary [from GitHub](https://github.com/okteto/okteto/releases).

#### Which binary should I download?

First of all you need to check your OS and architecture to download the correct binary. You can check your OS by running:

```console
uname
```

You will also need to know the architecture which can be found by executing:

```console
uname -m
```

#### How to install a binary

##### Linux/Mac

You must give permissions to the binary by executing the instruction:

```console
chmod u+x $OKTETO_BIN
```

Then you have to move the `$OKTETO_BIN` to your `$PATH` by executing.

```console
mv -f $OKTETO_BIN /usr/local/bin/okteto
```

### Installing the latest okteto release candidate

You can test the latest features by installing our release candidate. Please note that the changes in these versions do not necessarily work with the latest stable version of okteto.

#### Linux/Mac

```console
curl https://beta.okteto.com -sSfL | sh
```
