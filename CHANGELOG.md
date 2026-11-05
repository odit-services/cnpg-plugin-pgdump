# Changelog

## Version History

* [v0.4.0](#v0.4.0)
* [v0.3.3](#v0.3.3)
* [v0.3.2](#v0.3.2)
* [v0.3.1](#v0.3.1)
* [v0.3.0](#v0.3.0)
* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.4.0"></a>
### [v0.4.0](/compare/v0.3.3...v0.4.0)

> 2026-11-05

#### 🩹 Fixes

* **ci:** cache trivy db on runner disk, skip gha cache
* **ci:** drop gha docker cache
* **docker:** cross-compile for target arch, drop gosu, upgrade alpine packages

#### 🏡 Chore

* **deps:** bump go dependencies, go 1.27, pull fresh base images

#### 🚀 Enhancements

* **release:** attach trivy report and sbom to github release
* **workflows:** align pipelines with s3ops/ghops (ci, test, lint, releases, renovate)


<a name="v0.3.3"></a>
### [v0.3.3](/compare/v0.3.2...v0.3.3)

> 2026-07-21

#### 🩹 Fixes

* emit backup status once per backup

#### 🏡 Chore

* **release:** prepare v0.3.3


<a name="v0.3.2"></a>
### [v0.3.2](/compare/v0.3.1...v0.3.2)

> 2026-07-20

#### 🩹 Fixes

* avoid redundant plugin status reconciles

#### 🏡 Chore

* **release:** prepare v0.3.2


<a name="v0.3.1"></a>
### [v0.3.1](/compare/v0.3.0...v0.3.1)

> 2026-07-01

#### 🩹 Fixes

* set CNPG backup status phase

#### 🏡 Chore

* **release:** prepare v0.3.1


<a name="v0.3.0"></a>
### [v0.3.0](/compare/v0.2.0...v0.3.0)

> 2026-07-01

#### 🩹 Fixes

* skip inaccessible databases during discovery

#### 🏡 Chore

* **release:** prepare v0.3.0

#### 🚀 Enhancements

* configure backup user and inaccessible database handling

#### ✅ Tests

* run e2e in delegated rootless podman scope


<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 🏡 Chore

* **release:** prepare v0.2.0

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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

* [v0.3.3](#v0.3.3)
* [v0.3.2](#v0.3.2)
* [v0.3.1](#v0.3.1)
* [v0.3.0](#v0.3.0)
* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.3.3"></a>
### [v0.3.3](/compare/v0.3.2...v0.3.3)

> 2026-07-21

#### 🩹 Fixes

* emit backup status once per backup


<a name="v0.3.2"></a>
### [v0.3.2](/compare/v0.3.1...v0.3.2)

> 2026-07-20

#### 🩹 Fixes

* avoid redundant plugin status reconciles

#### 🏡 Chore

* **release:** prepare v0.3.2


<a name="v0.3.1"></a>
### [v0.3.1](/compare/v0.3.0...v0.3.1)

> 2026-07-01

#### 🩹 Fixes

* set CNPG backup status phase

#### 🏡 Chore

* **release:** prepare v0.3.1


<a name="v0.3.0"></a>
### [v0.3.0](/compare/v0.2.0...v0.3.0)

> 2026-07-01

#### 🩹 Fixes

* skip inaccessible databases during discovery

#### 🏡 Chore

* **release:** prepare v0.3.0

#### 🚀 Enhancements

* configure backup user and inaccessible database handling

#### ✅ Tests

* run e2e in delegated rootless podman scope


<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 🏡 Chore

* **release:** prepare v0.2.0

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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

* [v0.3.2](#v0.3.2)
* [v0.3.1](#v0.3.1)
* [v0.3.0](#v0.3.0)
* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.3.2"></a>
### [v0.3.2](/compare/v0.3.1...v0.3.2)

> 2026-07-20

#### 🩹 Fixes

* avoid redundant plugin status reconciles


<a name="v0.3.1"></a>
### [v0.3.1](/compare/v0.3.0...v0.3.1)

> 2026-07-01

#### 🩹 Fixes

* set CNPG backup status phase

#### 🏡 Chore

* **release:** prepare v0.3.1


<a name="v0.3.0"></a>
### [v0.3.0](/compare/v0.2.0...v0.3.0)

> 2026-07-01

#### 🩹 Fixes

* skip inaccessible databases during discovery

#### 🏡 Chore

* **release:** prepare v0.3.0

#### 🚀 Enhancements

* configure backup user and inaccessible database handling

#### ✅ Tests

* run e2e in delegated rootless podman scope


<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 🏡 Chore

* **release:** prepare v0.2.0

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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

* [v0.3.1](#v0.3.1)
* [v0.3.0](#v0.3.0)
* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.3.1"></a>
### [v0.3.1](/compare/v0.3.0...v0.3.1)

> 2026-07-01

#### 🩹 Fixes

* set CNPG backup status phase


<a name="v0.3.0"></a>
### [v0.3.0](/compare/v0.2.0...v0.3.0)

> 2026-07-01

#### 🩹 Fixes

* skip inaccessible databases during discovery

#### 🏡 Chore

* **release:** prepare v0.3.0

#### 🚀 Enhancements

* configure backup user and inaccessible database handling

#### ✅ Tests

* run e2e in delegated rootless podman scope


<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 🏡 Chore

* **release:** prepare v0.2.0

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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

* [v0.3.0](#v0.3.0)
* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.3.0"></a>
### [v0.3.0](/compare/v0.2.0...v0.3.0)

> 2026-07-01

#### 🩹 Fixes

* skip inaccessible databases during discovery

#### 🚀 Enhancements

* configure backup user and inaccessible database handling

#### ✅ Tests

* run e2e in delegated rootless podman scope


<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 🏡 Chore

* **release:** prepare v0.2.0

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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

* [v0.2.0](#v0.2.0)
* [v0.1.2](#v0.1.2)
* [v0.1.1](#v0.1.1)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.2.0"></a>
### [v0.2.0](/compare/v0.1.2...v0.2.0)

> 2026-06-26

#### 🩹 Fixes

* **ci:** yaml syntax in release-notes heredoc block

#### 📖 Documentation

* document one-time (ad-hoc) Backup in addition to ScheduledBackup
* add MIT license file

#### 🚀 Enhancements

* **config:** add bucket_secret_name / bucket_secret_key parameters
* **e2e:** seed sample data and add optional restore from S3 test


<a name="v0.1.2"></a>
### [v0.1.2](/compare/v0.1.1...v0.1.2)

> 2026-06-26

#### 🩹 Fixes

* correct changelog extraction and remove duplicate entries

#### 🏡 Chore

* **release:** prepare v0.1.2

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



