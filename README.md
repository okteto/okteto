# cnd

CLI to manage cloud native environments 

# Installation

```
go get github.com/okteto/cnd
```

# Usage

In order to create a dev environment, execute:

```
cnd up -f cnd.yml
```

You can find a `cnd.yml` example in `sample/cnd.yml`. 
`cnd.yml` defines the deployment and service to be swapped for development, and the local shared folder.


In order to destroy a dev environment, execute:

```
cnd down -f cnd.yml
```