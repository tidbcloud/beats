# Filebeat on TiDB Cloud

- A **Kubernetes label auto-discovery based** TiDB module for cloud-native TiDB clusters.
- A **file-path based** TiDB module covering TiDB and its ecosystem tools, such as PD, TiDB, TiKV, TiFlash, TiCDC, monitor, backup&restore, data migration, and ng-monitoring.
- A **file-path based** TiKV module covering TiKV and PD only.
- Based on the `v7.12` community release (to maintain compatible with AWS Opensearch `v1.x.x`).

> What is filebeat modules?
>
> A filebeat module is a user-friendly configuration interface which abstracts tedious [inputs](https://www.elastic.co/guide/en/beats/filebeat/7.17/configuration-filebeat-options.html) configurations.
>
> It also provides index schemas and lifecycle policies , which are auto-generated from the `fields.yml`, to elasticsearch.

## Get Started

First, get the latest version of the image at [my personal docker hub](https://hub.docker.com/repository/docker/sabaping/filebeat-oss-tidb-module).

Then refer to [从零开始体验 Filebeat TiDB Module](https://pingcap.feishu.cn/docs/doccnB7i1WOaRUBsrb6pojQu6eb).

## Local Development

### Prerequisite: Install Build Tool `mage`

```shell
export PATH=$PATH:$(go env GOPATH)/bin
go install github.com/magefile/mage
```

### Debug the Script Processor Separately

- Configure the filebeat to accept stdin input and output results to stdout
- Add your script processor

Like this:

```yaml
filebeat.inputs:
  - type: stdin
    multiline.type: pattern
    multiline.pattern: '^# Time: '
    multiline.negate: true
    multiline.match: after
    multiline.timeout: 1s
processors:
  - script:
      lang: javascript
      id: tidb_slow_log_parser
      params: { }
      source: >
        # your js scripts here
output.console:
  pretty: true
path.home: ./__local_home
logging.level: info
logging.metrics.enabled: false
```

Use this configuration to start filebeat process.

### Prepare a Minimal Elasticsearch Cluster

Use docker-compose to start an elasticsearch instance and a kibana instance.

`docker-compose.yml`:

```yaml
version: '2.2'
services:
  es01:
    image: docker.elastic.co/elasticsearch/elasticsearch:7.17.0
    container_name: es01
    environment:
      - discovery.type=single-node
    ports:
      - 9200:9200
      - 9300:9300
    networks:
      - elastic
  kib01:
    image: docker.elastic.co/kibana/kibana:7.17.0
    container_name: kib01
    ports:
      - 5601:5601
    environment:
      ELASTICSEARCH_URL: http://es01:9200
      ELASTICSEARCH_HOSTS: '["http://es01:9200"]'
    networks:
      - elastic
networks:
  elastic:
    driver: bridge
```

### Run Tests

All following steps are under `./filebeat` directory.

```shell
# Just run once
make clean
make python-env
source ./build/python-env/bin/activate
make filebeat.test
# Run after each time module changing
make update
# Start tests
GENERATE=1 INTEGRATION_TESTS=1 BEAT_STRICT_PERMS=false TESTING_FILEBEAT_MODULES=tikv pytest tests/system/test_modules.py
```

### Get Records from the Elasticsearch Instance

```shell
curl -X GET --location "http://localhost:9200/test-filebeat-modules/_search"
```

### View Test Configs and Logs

Test results locate at `./build/system-tests/run/`

### Build and Publish Docker Image Locally

First, bump the version number if needed.

```shell
export VERSION=7.12.X
# Under repo root directory
./dev-tools/set_version ${VERSION}
```

All following steps are under `./filebeat` directory.

```shell
# Focus on filebeat sub-module.
cd filebeat/

# Clean up first.
make clean

export VERSION=$(../dev-tools/get_version)

# PACKAGES and PLATFORMS is used by beats makefile(magefile).
# DOCKER_DEFAULT_PLATFORM is used by docker build command to force the build platform.
PACKAGES="docker" PLATFORMS="linux/amd64" DOCKER_DEFAULT_PLATFORM="linux/amd64" make release
docker tag docker.elastic.co/beats/filebeat-oss:${VERSION} sabaping/filebeat-oss-tidb-module:${VERSION}-amd64
PACKAGES="docker" PLATFORMS="linux/arm64" DOCKER_DEFAULT_PLATFORM="linux/arm64" make release
docker tag docker.elastic.co/beats/filebeat-oss:${VERSION} sabaping/filebeat-oss-tidb-module:${VERSION}-arm64

# Push to docker hub.
docker push sabaping/filebeat-oss-tidb-module:${VERSION}-amd64
docker push sabaping/filebeat-oss-tidb-module:${VERSION}-arm64

# Merge to a single multi-arch image.
docker manifest create sabaping/filebeat-oss-tidb-module:${VERSION} --amend sabaping/filebeat-oss-tidb-module:${VERSION}-arm64 --amend sabaping/filebeat-oss-tidb-module:${VERSION}-amd64
docker manifest push sabaping/filebeat-oss-tidb-module:${VERSION}
```

## References

### Branch Policy

All developments are under the `tidbcloud` namespace.

- `tidbcloud/master`: The default branch which maps to the nightly dev environment.

### TiDB Cluster Components

TiDB operator tags each pod with the [label `app.kubernetes.io/component`](https://github.com/pingcap/tidb-operator/blob/master/pkg/apis/label/label.go#L31).

[Possible components](https://github.com/pingcap/tidb-operator/blob/master/pkg/apis/label/label.go#L122) are:

```text
pd
tidb
tikv
tiflash
ticdc
monitor
clean
restore
backup
dm-master
dm-worker
ng-monitoring
```

The infra-api might deploy a tidb-lightning job. This job is another component:

```text
tidb-lightning
```

These component values are also a part of pod name. Filebeat could use them to discover log files.


---

# Beats - The Lightweight Shippers of the Elastic Stack

The [Beats](https://www.elastic.co/products/beats) are lightweight data
shippers, written in Go, that you install on your servers to capture all sorts
of operational data (think of logs, metrics, or network packet data). The Beats
send the operational data to Elasticsearch, either directly or via Logstash, so
it can be visualized with Kibana.

By "lightweight", we mean that Beats have a small installation footprint, use
limited system resources, and have no runtime dependencies.

This repository contains
[libbeat](https://github.com/elastic/beats/tree/master/libbeat), our Go
framework for creating Beats, and all the officially supported Beats:

Beat  | Description
--- | ---
[Auditbeat](https://github.com/elastic/beats/tree/master/auditbeat) | Collect your Linux audit framework data and monitor the integrity of your files.
[Filebeat](https://github.com/elastic/beats/tree/master/filebeat) | Tails and ships log files
[Functionbeat](https://github.com/elastic/beats/tree/master/x-pack/functionbeat) | Read and ships events from serverless infrastructure.
[Heartbeat](https://github.com/elastic/beats/tree/master/heartbeat) | Ping remote services for availability
[Journalbeat](https://github.com/elastic/beats/tree/master/journalbeat) | Read and ships event from Journald.
[Metricbeat](https://github.com/elastic/beats/tree/master/metricbeat) | Fetches sets of metrics from the operating system and services
[Packetbeat](https://github.com/elastic/beats/tree/master/packetbeat) | Monitors the network and applications by sniffing packets
[Winlogbeat](https://github.com/elastic/beats/tree/master/winlogbeat) | Fetches and ships Windows Event logs

In addition to the above Beats, which are officially supported by
[Elastic](https://elastic.co), the community has created a set of other Beats
that make use of libbeat but live outside of this Github repository. We maintain
a list of community Beats
[here](https://www.elastic.co/guide/en/beats/libbeat/master/community-beats.html).

## Documentation and Getting Started

You can find the documentation and getting started guides for each of the Beats
on the [elastic.co site](https://www.elastic.co/guide/):

* [Beats platform](https://www.elastic.co/guide/en/beats/libbeat/current/index.html)
* [Auditbeat](https://www.elastic.co/guide/en/beats/auditbeat/current/index.html)
* [Filebeat](https://www.elastic.co/guide/en/beats/filebeat/current/index.html)
* [Functionbeat](https://www.elastic.co/guide/en/beats/functionbeat/current/index.html)
* [Heartbeat](https://www.elastic.co/guide/en/beats/heartbeat/current/index.html)
* [Journalbeat](https://www.elastic.co/guide/en/beats/journalbeat/current/index.html)
* [Metricbeat](https://www.elastic.co/guide/en/beats/metricbeat/current/index.html)
* [Packetbeat](https://www.elastic.co/guide/en/beats/packetbeat/current/index.html)
* [Winlogbeat](https://www.elastic.co/guide/en/beats/winlogbeat/current/index.html)


## Getting Help

If you need help or hit an issue, please start by opening a topic on our
[discuss forums](https://discuss.elastic.co/c/beats). Please note that we
reserve GitHub tickets for confirmed bugs and enhancement requests.

## Downloads

You can download pre-compiled Beats binaries, as well as packages for the
supported platforms, from [this page](https://www.elastic.co/downloads/beats).

## Contributing

We'd love working with you! You can help make the Beats better in many ways:
report issues, help us reproduce issues, fix bugs, add functionality, or even
create your own Beat.

Please start by reading our [CONTRIBUTING](CONTRIBUTING.md) file.

If you are creating a new Beat, you don't need to submit the code to this
repository. You can simply start working in a new repository and make use of the
libbeat packages, by following our [developer
guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).
After you have a working prototype, open a pull request to add your Beat to the
list of [community
Beats](https://github.com/elastic/beats/blob/master/libbeat/docs/communitybeats.asciidoc).

## Building Beats from the Source

See our [CONTRIBUTING](CONTRIBUTING.md) file for information about setting up
your dev environment to build Beats from the source.

## Snapshots

For testing purposes, we generate snapshot builds that you can find [here](https://beats-ci.elastic.co/job/Beats/job/packaging/job/master/lastSuccessfulBuild/gcsObjects/). Please be aware that these are built on top of master and are not meant for production.

## CI

### PR Comments

It is possible to trigger some jobs by putting a comment on a GitHub PR.
(This service is only available for users affiliated with Elastic and not for open-source contributors.)

* [beats][]
  * `jenkins run the tests please` or `jenkins run tests` or `/test` will kick off a default build.
  * `/test macos` will kick off a default build with also the `macos` stages.
  * `/test <beat-name>` will kick off the default build for the given PR in addition to the `<beat-name>` build itself.
  * `/test <beat-name> for macos` will kick off a default build with also the `macos` stage for the `<beat-name>`.
* [apm-beats-update][]
  * `/run apm-beats-update`
* [apm-beats-packaging][]
  * `/package` or `/packaging` will kick of a build to generate the packages for beats.
* [apm-beats-tester][]
  * `/beats-tester` will kick of a build to validate the generated packages.

### PR Labels

It's possible to configure the build on a GitHub PR by labelling the PR with the below labels

* `<beat-name>` to force the following builds to run the stages for the `<beat-name>`
* `macOS` to force the following builds to run the `macos` stages.

[beats]: https://beats-ci.elastic.co/job/Beats/job/beats/
[apm-beats-update]: https://beats-ci.elastic.co/job/Beats/job/apm-beats-update/
[apm-beats-packaging]: https://beats-ci.elastic.co/job/Beats/job/packaging/
[apm-beats-tester]: https://beats-ci.elastic.co/job/Beats/job/beats-tester/
