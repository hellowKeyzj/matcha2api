export default {
  packages: {
    title: 'Package Management',
    description: 'Manage resource packages, reviews, lifecycle, plan and permission bindings, download audits, and storage.',
    actions: {
      refresh: 'Refresh',
      storageConfig: 'Storage Config',
      create: 'New Package',
      edit: 'Edit',
      audit: 'Audit',
      recordDownload: 'Record Download',
      bindings: 'Bind',
      approve: 'Approve',
      reject: 'Reject',
      takeDown: 'Take Down',
      delete: 'Delete',
      saveBindings: 'Save Bindings',
      saveConfig: 'Save Config'
    },
    filters: {
      searchPlaceholder: 'Search name, version, description, or storage key',
      allStatuses: 'All Statuses'
    },
    summary: {
      total: 'Total Packages',
      pendingReview: 'Pending Review',
      approved: 'Approved',
      downloads: 'Downloads'
    },
    cloudKey: {
      title: 'Cloud package key',
      description: 'Used for package cloudEnvelope upload and later authorized installs. The private key stays on the server.',
      configured: 'Configured',
      notConfigured: 'Not configured',
      generate: 'Generate Key',
      generating: 'Generating',
      sources: {
        database: 'Database',
        config: 'Config file',
        static: 'Static injection',
        none: 'Not configured'
      }
    },
    empty: {
      title: 'No packages',
      description: 'Create a package first, then bind it to plans or permissions.'
    },
    fields: {
      name: 'Name',
      version: 'Version',
      description: 'Description',
      status: 'Status',
      sizeBytes: 'Size (bytes)',
      fileURL: 'File URL',
      storageKey: 'Storage Key',
      checksum: 'Checksum',
      planBindings: 'Plan Bindings',
      permissionBindings: 'Permission Bindings',
      download: 'Download',
      storage: 'Storage',
      reviewNote: 'Review Note',
      takedownReason: 'Takedown Reason',
      driver: 'Driver',
      localPath: 'Local Path',
      publicBaseURL: 'Public Base URL',
      endpoint: 'Endpoint',
      region: 'Region',
      bucket: 'Bucket',
      accessKeyID: 'Access Key ID',
      secretAccessKey: 'Secret Access Key',
      prefix: 'Prefix',
      presignExpiryHours: 'Presign Expiry (hours)',
      reuseBackupS3: 'Reuse backup S3 credentials',
      forcePathStyle: 'Force path style',
      time: 'Time',
      package: 'Package',
      user: 'User',
      ip: 'IP',
      userAgent: 'User Agent'
    },
    dialogs: {
      create: 'New Package',
      edit: 'Edit Package',
      bindings: 'Plan / Permission Bindings',
      storage: 'Package Storage Config',
      audits: 'Download Audit'
    },
    placeholders: {
      version: '1.0.0',
      fileURL: 'https://...',
      storageKey: 'packages/name.zip',
      checksum: 'sha256:...',
      commaPlans: 'Comma separated, e.g. basic,pro',
      commaPermissions: 'Comma separated, e.g. package.download',
      commaSeparated: 'Comma separated',
      publicBaseURL: 'https://cdn.example.com',
      bucketInherited: 'Leave empty to inherit backup bucket',
      prefix: 'matcha-packages',
      endpoint: 'https://<account_id>.r2.cloudflarestorage.com',
      region: 'auto',
      secretConfigured: 'Configured; leave empty to keep unchanged'
    },
    status: {
      draft: 'Draft',
      pending_review: 'Pending Review',
      approved: 'Approved',
      rejected: 'Rejected',
      taken_down: 'Taken Down'
    },
    storageDrivers: {
      local: 'Local',
      s3: 'S3'
    },
    labels: {
      downloadCount: '{count} downloads',
      allPackages: 'All Packages',
      noAudits: 'No audit records'
    },
    prompts: {
      approveNote: 'Review note (optional)',
      rejectNote: 'Rejection reason (optional)',
      takedownReason: 'Takedown reason (optional)'
    },
    confirms: {
      delete: 'Delete package "{name}"?'
    },
    messages: {
      saved: 'Package saved',
      bindingsSaved: 'Bindings saved',
      approved: 'Package approved',
      rejected: 'Package rejected',
      takenDown: 'Package taken down',
      deleted: 'Package deleted',
      auditRecorded: 'Download audit recorded',
      storageSaved: 'Storage config saved',
      cloudKeyGenerated: 'Cloud package key generated'
    },
    errors: {
      fallback: 'Operation failed',
      loadFailed: 'Failed to load packages',
      storageLoadFailed: 'Failed to load storage config',
      saveFailed: 'Failed to save package',
      bindingsSaveFailed: 'Failed to save bindings',
      reviewFailed: 'Failed to review package',
      takedownFailed: 'Failed to take down package',
      deleteFailed: 'Failed to delete package',
      auditRecordFailed: 'Failed to record download audit',
      auditLoadFailed: 'Failed to load download audit',
      storageSaveFailed: 'Failed to save storage config',
      cloudKeyLoadFailed: 'Failed to load cloud package key status',
      cloudKeyGenerateFailed: 'Failed to generate cloud package key',
      PACKAGE_REGISTRY_UNAVAILABLE: 'Package registry is temporarily unavailable',
      MATCHA_PACKAGE_NOT_FOUND: 'Package not found or already deleted',
      MATCHA_PACKAGE_VERSION_NOT_FOUND: 'Package version not found or already deleted',
      MATCHA_PACKAGE_ENTITLEMENT_NOT_FOUND: 'The current user is not entitled to this package',
      MATCHA_PACKAGE_EXISTS: 'Package already exists',
      MATCHA_PACKAGE_VERSION_EXISTS: 'Package version already exists',
      MATCHA_PACKAGE_STORAGE_NOT_CONFIGURED: 'Package storage is not configured',
      MATCHA_PACKAGE_BACKUP_S3_NOT_CONFIGURED: 'Backup S3 is not configured, so backup credentials cannot be reused',
      MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED: 'Cloud package key is not configured',
      MATCHA_PACKAGE_CLOUD_KEY_EXISTS: 'Cloud package key is already configured and cannot be generated again',
      SECRET_ENCRYPTION_KEY_NOT_CONFIGURED: 'A fixed encryption key is not configured, so server secrets cannot be saved',
      MATCHA_PACKAGE_CLOUD_ENVELOPE_REQUIRED: 'Export the package again for the cloud, then upload it',
      MATCHA_PACKAGE_VERSION_NOT_PUBLISHABLE: 'This package version cannot be published',
      MATCHA_PACKAGE_VERSION_NOT_DOWNLOADABLE: 'This package version is not downloadable',
      INVALID_MATCHA_PACKAGE: 'Invalid package parameters',
      INVALID_MATCHA_PACKAGE_OBJECT: 'Invalid package object parameters',
      INVALID_MATCHA_PACKAGE_OBJECT_KEY: 'Invalid package storage key',
      INVALID_MATCHA_PACKAGE_ENTITLEMENT: 'Invalid package entitlement parameters',
      INVALID_MATCHA_PACKAGE_BILLING_POLICY: 'Invalid package billing policy parameters',
      MATCHA_PACKAGE_FORBIDDEN: 'You do not have permission to modify this package'
    }
  }
}
