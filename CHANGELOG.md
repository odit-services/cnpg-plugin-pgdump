# Changelog

## Version History

* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 📖 Documentation

* document supported postgres versions in readme and release body


<a name="v0.1.1"></a>
### [v0.1.1](/compare/v0.1.0...v0.1.1)

> 2026-06-26

#### 🏡 Chore

* **release:** prepare v0.1.1

#### 📖 Documentation

* explain cnpg plugin tls setup


<a name="v0.1.0"></a>
### v0.1.0

> 2026-06-26

#### 🩹 Fixes

* enable standalone plugin startup with mtls

#### 🏡 Chore

* add shikai release task
* wire e2e test workflow
* **release:** prepare v0.1.0

#### 📖 Documentation

* add ghcr quickstart
* show explicit s3 secret keys
* document taskfile and s3 secret refs
* add deployment and usage examples

#### 🚀 Enhancements

* select pg_dump by server version
* add timestamp object key placeholder
* support configurable backup object keys
* support per-backup s3 secrets
* implement cnpg pgdump backup plugin
* **workflows:** Added docker build workflow

#### 💅 Refactors

* remove deployment s3 configuration

#### ✅ Tests

* parallelize e2e postgres versions
* make e2e suite work with podman kind
* add kind cucumber e2e suite
* cover backup retention and config parsing



# Changelog

## Version History

* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.1.1"></a>
### [v0.1.1](/compare/v0.1.0...v0.1.1)

> 2026-06-26

#### 📖 Documentation

* explain cnpg plugin tls setup


<a name="v0.1.0"></a>
### v0.1.0

> 2026-06-26

#### 🩹 Fixes

* enable standalone plugin startup with mtls

#### 🏡 Chore

* add shikai release task
* wire e2e test workflow
* **release:** prepare v0.1.0

#### 📖 Documentation

* add ghcr quickstart
* show explicit s3 secret keys
* document taskfile and s3 secret refs
* add deployment and usage examples

#### 🚀 Enhancements

* select pg_dump by server version
* add timestamp object key placeholder
* support configurable backup object keys
* support per-backup s3 secrets
* implement cnpg pgdump backup plugin
* **workflows:** Added docker build workflow

#### 💅 Refactors

* remove deployment s3 configuration

#### ✅ Tests

* parallelize e2e postgres versions
* make e2e suite work with podman kind
* add kind cucumber e2e suite
* cover backup retention and config parsing



