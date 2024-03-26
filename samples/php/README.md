# Getting Started with Okteto and PHP

This tutorial will show you how to develop and debug a PHP application using Okteto

## Step 1: Deploy the PHP Sample App

Run the following command to deploy the PHP Sample App:

```bash
kubectl apply -f k8s.yml
```

```bash
deployment.apps/hello-world created
service/hello-world created
```

## Step 2: Activate your development container

The [dev section](https://www.okteto.com/docs/reference/okteto-manifest/#dev-object-optional) of the Okteto Manifest defines how to activate a development container for the PHP Sample App:

```yaml
dev:
  hello-world:
    image: okteto/php-getting-started:dev
    command: bash
    sync:
      - .:/app
    forward:
      - 8080:8080
    reverse:
      - 9000:9000
    volumes:
      - /root/.composer/cache
```

The `hello-world` key matches the name of the hello world Deployment. The meaning of the rest of fields is:

- `image`: the image used by the development container (built from this [Dockerfile](Dockerfile)).
- `command`: the start command of the development container.
- `sync`: the folders that will be synchronized between your local machine and the development container.
- `forward`: a list of ports to forward from your development container to localhost in your machine. This is needed to access the port 8080 of your application on localhost.
- `reverse`: a list of ports to reverse forward from your development container to your local machine. This is needed by the PHP remote debugger.
- `volumes`: a list of paths in your development container to be mounted as persistent volumes. For example, this is useful to persist the Composer cache.

Also, note that there is a `.stignore` file to indicate which files shouldn't be synchronized to your development container.
This is useful to avoid synchronizing binaries, build artifacts, or git metadata.

Next, execute the following command to activate your development container:

```bash
okteto up
```

```bash
 ✓  Images successfully pulled
 ✓  Files synchronized
    Namespace: cindy
    Name:      hello-world
    Forward:   8080 -> 8080
    Reverse:   9000 <- 9000

Welcome to your development container. Happy coding!
cindy:hello-world app>
```

Working in your development container is the same as working on your local machine.
Start the application by running the following command:

```bash
cindy:hello-world app> php -S 0.0.0.0:8080
```

```bash
[Tue Jul  5 21:04:55 2022] PHP 8.2.0 Development Server (http://0.0.0.0:8080) started
```

Open your browser and load the page `http://localhost:8080` to test that your application is running.
You should see the message:

```bash
Hello world!
```

## Step 3: Remote Development with Okteto

Open the `index.php` file in your favorite local IDE and modify the response message on line 2 to be _Hello world from Okteto!_. Save your changes.

```php
<?php
$message = "Hello World from Okteto!";
echo($message);
```

Go back to the browser and reload the page. Your code changes were instantly applied. No commit, build or push required 😎!

## Step 4: Remote debugging with Okteto

Okteto enables you to debug your applications directly from your favorite IDE. Let's take a look at how that works with [PHPStorm](https://www.jetbrains.com/phpstorm/), one of the most popular IDEs for PHP development.

If you haven't already, fire up PHP Storm and load this project there. Once the project is loaded, open `index.php` and set a breakpoint in `line 2`. Click on the `Start Listen PHP Debug Connections` button on the PhpStorm toolbar.

Go back to your browser and reload the page. The execution will automatically halt at the breakpoint.

> If this is the first time you debug this application, the IDE will ask you to confirm the source mapping configuration. Verify the values and click `ok` to continue.

At this point, you're able to inspect the request object, the current values of everything, the contents of `$_SERVER` variable, etc.

<img align="left" src="images/php-halt.png">

Your code is executing in Okteto, but you can debug it from your local machine without any extra services or tools. Pretty cool no? 😉
