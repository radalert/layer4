# Nudger

Nudger is a New Relic metrics importer for Rad Alert.

It periodically queries the New Relic REST API (v2), and dispatches gathered
metrics to the Pacemaker for analysis.

## Deploying

 1. Make your changes, `git commit` them.
 2. `git push` your changes
 3. Watch the changes get [Continuously Deployed via Jenkins](http://ci.radalert.io/job/nudger/lastBuild/consoleFull)

The CD pipeline builds a Docker image and deploys it via Ansible.

## Developing

``` bash
git clone git@github.com:radalert/layer4.git
cd layer4/nudger
cp nudger.sample.json nudger.test.json
foreman start
```

## Debugging

Nudger is run out of a Docker container on our web infrastructure.

You can tail the logs to see what it's doing:

```
ssh 23.251.149.80
docker logs -f nudger
```
