# Taut

Taut is a Slack chat bot for Rad Alert.

It acts as a gateway between Rad Alert's Pacemaker and an organisation's Slack
account.

Taut can:

 - Send alerts from Rad Alert to Slack (courtesy of web hooks).
 - Respond to mentions in Slack to provide more alert context, and push
   feedback to the Pacemaker.

## Deploying

 1. Make your changes, `git commit` them.
 2. `git push` your changes
 3. Watch the changes get [Continuously Deployed via Jenkins](http://ci.radalert.io/job/taut/lastBuild/consoleFull)

The CD pipeline builds a Docker image and deploys it via Ansible.

## Developing

``` bash
git clone git@github.com:radalert/layer4.git
cd layer4/taut
cp taut.sample.json taut.test.json
foreman start
```

## Debugging

Taut is run out of a Docker container on our web infrastructure.

You can tail the logs to see what it's doing:

```
ssh 23.251.149.80
docker logs -f taut
```
