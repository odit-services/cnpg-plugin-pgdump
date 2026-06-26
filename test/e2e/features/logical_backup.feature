Feature: Logical pg_dump backups to RustFS
  CloudNativePG clusters can use the pg_dump plugin through ScheduledBackup CRDs.

  Scenario: ScheduledBackup uploads one dump per database for each PostgreSQL version
    Given a kind cluster for pgdump e2e tests
    And CloudNativePG is installed
    And RustFS is running as the S3 target
    And the pgdump plugin is deployed
    When I run logical backups for the configured PostgreSQL versions
    Then every PostgreSQL version should have uploaded dumps to RustFS
    And I should be able to restore dumps from S3
