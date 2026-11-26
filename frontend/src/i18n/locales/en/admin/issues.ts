export default {
  issues: {
    searchPlaceholder: 'ID, title, reporter, version, or label',
    allIssues: 'All feedback',
    totalCount: '{count} total',
    selectedCount: '{count} selected',
    selectIssue: 'Select #{id}',
    empty: 'No matching feedback',
    listLoadFailed: 'Could not load feedback',
    reporterFilter: 'Reporter: {email}',
    previewAttachment: 'Preview {name}',
    evidence: {
      images: 'Screenshots: {count}',
      videos: 'Recordings: {count}',
      notes: 'Notes: {count}'
    },
    status: {
      all: 'All',
      open: 'Open',
      in_progress: 'In Progress',
      resolved: 'Resolved',
      closed: 'Closed'
    },
    kind: {
      all: 'All types',
      bug: 'Bug',
      feature: 'Suggestion',
      experience: 'Experience'
    },
    severity: {
      all: 'All levels',
      blocker: 'Blocker',
      major: 'Major',
      minor: 'Minor'
    },
    filters: {
      diagnosticsOnly: 'With diagnostics'
    },
    actions: {
      bulk: {
        in_progress: 'Start triage',
        resolved: 'Resolve',
        closed: 'Close selected'
      },
      clearReporter: 'Clear reporter filter: {email}',
      copyRef: 'Copy #ID',
      copiedRef: 'Copied #{id}',
      copyEnv: 'Copy',
      copiedEnv: 'Copied client snapshot',
      copySha: 'Copy SHA-256',
      copiedSha: 'Copied SHA-256',
      download: 'Download',
      downloadDiagnostics: 'Download ZIP',
      inspectManifest: 'File list',
      loadingManifest: 'Reading archive',
      manifestEmpty: 'Archive has no files',
      addNote: 'Add note',
      noteAndResolve: 'Note & resolve',
      noteAndClose: 'Note & close',
      prev: 'Previous',
      next: 'Next',
      save: 'Save',
      addLabel: 'Add label'
    },
    detail: {
      noDescription: 'No description',
      internalNote: 'Internal note',
      notePlaceholder: 'Add an internal note',
      fixVersionPlaceholder: 'Fix version',
      statusLabel: 'Status',
      severityLabel: 'Severity',
      assigneeLabel: 'Assignee',
      assigneePlaceholder: 'Assignee name',
      labelsLabel: 'Labels',
      customLabelPlaceholder: 'Custom label',
      fixVersionLabel: 'Fix version',
      envLabel: 'Client snapshot',
      envVersion: 'Version',
      envOs: 'OS',
      envArch: 'Arch',
      envRoute: 'Route',
      envLocale: 'Locale',
      envDiagnostics: 'Diagnostics',
      notAttached: 'None',
      reporterLabel: 'Reporter',
      viewReporterIssues: 'View submissions ({count})',
      loadFailed: 'Could not load this report',
      updateFailed: 'Update failed',
      mediaLoadFailed: 'Preview unavailable',
      manifestLoadFailed: 'Could not read the file list',
      downloadFailed: 'Download failed'
    },
    timeline: {
      created: 'submitted from desktop client',
      createdWithDiag: 'submitted with diagnostics',
      statusChanged: 'changed status from {from} to {to}',
      statusChangedBatch: 'batch-changed status from {from} to {to}',
      severityChanged: 'changed severity from {from} to {to}',
      assigneeChanged: 'assigned to {to}',
      assigneeCleared: 'cleared assignee',
      fixVersionChanged: 'linked fix version {to}',
      fixVersionCleared: 'cleared fix version',
      labelsUpdated: 'updated labels'
    }
  }
}
