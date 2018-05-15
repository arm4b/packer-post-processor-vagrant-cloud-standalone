# vagrant-cloud-standalone
[![Build Status](https://travis-ci.org/armab/packer-post-processor-vagrant-cloud-standalone.svg?branch=master)](https://travis-ci.org/armab/packer-post-processor-vagrant-cloud-standalone)
[![GitHub release](https://img.shields.io/github/release/armab/packer-post-processor-vagrant-cloud-standalone.svg)](https://github.com/armab/packer-post-processor-vagrant-cloud-standalone/releases/latest)
[![License](https://img.shields.io/github/license/armab/packer-post-processor-vagrant-cloud-standalone.svg?style=flat)](LICENSE)

Packer post-processor plugin for uploading artifacts to [Vagrant Cloud](https://vagrantcloud.com/) from the input filepath.

## Description
This post-processor is a fork of core Packer's [vagrant-cloud](https://www.packer.io/docs/post-processors/vagrant-cloud.html)
plugin ([source code](https://github.com/hashicorp/packer/tree/master/post-processor/vagrant-cloud)).

While original `vagrant-cloud` plugin requires artifact produced from the previous `vagrant` post-processor
involving entire build stage, forked version `vagrant-cloud-standalone` just uploads artifact to
Vagrant Cloud directly by input filepath, hence standalone.

It can be useful to split Packer build/deploy stage if you prefer to divide CI/CD or when artifact
was already produced before.

## Installation
Packer supports pluggable mechanism. Please read the following documentation to understand how to install this plugin:

https://www.packer.io/docs/extend/plugins.html

You can download binary built for your architecture from [Github Releases](https://github.com/armab/packer-post-processor-vagrant-cloud-standalone/releases).

## Usage
Here is a simple example of `vagrant_deploy.json`:

```json
{
  "variables": {
    "description": "Packer template for deploying a .box artifact to Vagrant CLoud",
    "cloud_token": "{{ env `VAGRANT_CLOUD_TOKEN` }}"
  },
  "builders": [
    {
      "type": "file",
      "content": "Do nothing, Packer just requires at least one builder to be present",
      "target": "/dev/null"
    }
  ],
  "post-processors": [
    {
      "type": "vagrant-cloud-standalone",
      "access_token": "{{user `cloud_token`}}",
      "box_tag": "ubuntu/xenial64",
      "provider": "virtualbox",
      "version": "20180130.0.0",
      "artifact": "builds/ubuntu-xenial_v20180130.0.0.box"
    }
  ]
}
```

It will verify the box, create new Version, Provider, Upload the provided .box and then Release new version in Vagrant Cloud.

### Configuration
Configuration is the same as original Packer's [vagrant-cloud](https://www.packer.io/docs/post-processors/vagrant-cloud.html).

A few settings were added to allow uploading .box artifact from the local file path:
- `provider` (string)
  - Box type, Vagrant provider like `virtualbox`, `vmware`, etc.
- `artifact` (string)
  - Path to artifact file `.box`. to deploy.
