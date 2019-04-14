# Get your code out there

Stop dealing with local development environments.

Okteto gives your a cloud environment to code and collaborate.

Edit in your favorite IDE and save to your filesystem. No commit or push required.

## Getting started with Okteto

This guide shows you how to get started with Okteto. It'll take less than 5 minutes to have you developing a full stack application directly in the cloud. While you're going through the tutorial, you might want to join [Okteto's slack community](https://okteto-community.slack.com/join/shared_invite/enQtNDg3MTMyMzA1OTg3LTY1NzE0MGM5YjMwOTAzN2YxZTU3ZjkzNTNkM2Y1YmJjMjlkODU5Mzc1YzY0OThkNWRhYzhkMTM2NWFlY2RkMDk") for technical support, or just to say hi 😸.

### Step 1: Install

The first thing you need to do is to login at https://cloud.okteto.com. This will automatically create an Okteto Space for you in the cloud.

### Step 2: Install the CLI

Install the Okteto CLI by running:

```console
curl https://get.okteto.com -sSfL | sh
```

### Step 3: Login from the CLI

```console
okteto login
```

### Step 4: Create your Okteto environment

Clone the samples repository and move to the `vote` folder:
```console
$ git clone https://github.com/okteto/cloud-samples
$ cd cloud-samples/vote
```

and now execute:

```console
okteto up
```

The `okteto up` command will create an Okteto Environment that automatically synchronizes and applies your local code changes. In the Okteto Terminal, execute:

```console
pip install -r requirements.txt
python app.py
```

Check that your application is running by going to [Okteto](https://cloud.okteto.com) and clicking the endpoint of your application.

As you can see, the app is failing because it requires a redis database running on your Okteto Environment. Go to 
[Okteto](https://cloud.okteto.com) and click the `+` buttom on the right, pick `Database` and then `Redis`. This will create a Redis Database in your Okteo Space. After a few seconds, check again your application and now you should have your python application up and running.

### Step 5: Develop directly in the cloud

Open `vote/app.py` in your favorite IDE. Change the `optionA` in line 8 from "Cats" to "Otters" and save your changes.

If you go back to the Okteto Trminal, you'll notice that flask already detected the code changes and reloaded your application.
```console
...
 * Detected change in '/usr/src/app/app.py', reloading
 * Restarting with stat
 * Debugger is active!
 * Debugger PIN: 778-756-428
```

Go back to the browser, and reload the application. Notice how your changes to your application are instantly applied 😎! 
