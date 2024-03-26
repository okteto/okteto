# Getting Started with Okteto and Python

This tutorial will show you how to develop and debug a Python application using Okteto

## Step 1: Deploy the ASP.NET Sample App

Run the following command to deploy the ASP.NET Sample App:

```bash
kubectl apply -f k8s.yml
```

```bash
deployment.apps/hello-world created
service/hello-world created
```

Create a port-forward to access the ASP.NET Sample App on localhost:

```bash
kubectl port-forward service/hello-world 8080:8080
```

In a different terminal, access you application by executing:

```bash
curl localhost:8080
```

```bash
Hello World!
```

## Step 2: Activate your development container

The [dev](reference/okteto-manifest.mdx#dev-object-optional) section defines how to activate a development container for the Python Sample App:

```yaml title="okteto.yml"
dev:
  hello-world:
    command: bash
    environment:
      - FLASK_ENV=development
    sync:
      - .:/usr/src/app
    reverse:
      - 9000:9000
    volumes:
      - /root/.cache/pip
```

The `hello-world` key matches the name of the hello world Deployment. The meaning of the rest of fields is:

- `command`: the start command of the development container.
- `sync`: the folders that will be synchronized between your local machine and the development container.
- `reverse`: a list of ports to reverse forward from your development container to your local machine
- `volumes`: a list of paths in your development container to be mounted as persistent volumes. This is useful to persist the pip cache.

Also, note that there is a `.stignore` file to indicate which files shouldn't be synchronized to your development container.
This is useful to avoid virtual environments, build artifacts, or git metadata.

Next, execute the following command to activate your development container:

```bash
$ okteto up
```

```bash
 ✓  Persistent volume successfully attached
 ✓  Images successfully pulled
 ✓  Files synchronized
    Namespace: cindy
    Name:      hello-world
    Reverse:   9000 <- 9000

Welcome to your development container. Happy coding!
cindy:hello-world app>
```

Working in your development container is the same as working on your local machine.
Start the application in development mode by running the following command:

```bash
cindy:hello-world app> python app.py
```

```
Starting hello-world server...
 * Serving Flask app "app" (lazy loading)
 * Environment: development
 * Debug mode: on
 * Running on http://0.0.0.0:8080/ (Press CTRL+C to quit)
```

Go back to the browser and reload the page to test that your application is running.

## Step 3: Remote Development with Okteto

Open the `app.py` file in your favorite local IDE and modify the response message on line 7 to be _Hello world from the cluster!_.
Save your changes.

```python
@app.route('/')
def hello_world():
    return 'Hello World from the cluster!'
}
```

Okteto will synchronize your changes to your development container.
Flask's auto-reloader will detect the changes automatically and restart the application with the new code.

```bash
 * Detected change in '/usr/src/app/app.py', reloading
 * Restarting with stat
Starting hello-world server...
 * Debugger is active!
 * Debugger PIN: 308-916-374
```

Go back to the browser and reload the page. Your code changes were instantly applied. No commit, build, or push required 😎!

## Step 4: Remote debugging with Okteto

Okteto enables you to debug your applications directly from your favorite IDE.
Let's take a look at how that works in one of python's most popular IDE's, [PyCharm](https://www.jetbrains.com/pycharm/).

:::note

For VS Code users, this [document](https://code.visualstudio.com/docs/python/debugging#_debugging-by-attaching-over-a-network-connection) explains how to configure the debugger with `debugpy`.

:::

First, open the project in PyCharm and remove the comments on `app.py` line `20`.

```python
if __name__ == '__main__':
  print('Starting hello-world server...')
  # comment out to use Pycharm's remote debugger
  attach()

  app.run(host='0.0.0.0', port=8080)
```

Second, launch the [Remote Debug Server](https://www.jetbrains.com/help/pycharm/remote-debugging-with-product.html) by clicking on the Debug button on the top right.
Ensure that the Debug Tool Window shows the `Waiting for process connection...` message. This message will be shown until you launch your app on the development container shell and it connects to the Debug Server.

```bash
Starting hello-world server...
 * Serving Flask app "app" (lazy loading)
 * Environment: development
 * Debug mode: on
 * Running on http://0.0.0.0:8080/ (Press CTRL+C to quit)
 * Restarting with stat
Starting hello-world server...
Connecting to debugger...
```

On your local machine, switch to the Debug Tool Window. Once the app connects it will show the connection to the pydev debugger.
Press the `resume` button to let the execution continue.

<img align="left" src="images/python-connected.png">

Add a breakpoint on `app.py`, line 10. Go back to the browser and reload the page.

The execution will halt at your breakpoint. You can then inspect the request, the available variables, etc.

<img align="left" src="images/python-debug.png">

Your code is executing in Okteto, but you can debug it from your local machine without any extra services or tools. Pretty cool no? 😉
