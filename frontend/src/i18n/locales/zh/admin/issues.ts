export default {
  issues: {
    searchPlaceholder: '编号、标题、账号、版本或标签',
    allIssues: '全部反馈',
    totalCount: '共 {count} 条',
    selectedCount: '已选 {count} 条',
    selectIssue: '选择 #{id}',
    empty: '没有匹配的反馈',
    listLoadFailed: '反馈列表加载失败',
    reporterFilter: '提交人：{email}',
    previewAttachment: '预览 {name}',
    evidence: {
      images: '截图：{count}',
      videos: '录屏：{count}',
      notes: '备注：{count}'
    },
    status: {
      all: '全部',
      open: '待处理',
      in_progress: '排查中',
      resolved: '已解决',
      closed: '已关闭'
    },
    kind: {
      all: '全部类型',
      bug: '问题',
      feature: '建议',
      experience: '体验'
    },
    severity: {
      all: '全部级别',
      blocker: '卡死阻断',
      major: '影响使用',
      minor: '轻微'
    },
    filters: {
      diagnosticsOnly: '仅含诊断包'
    },
    actions: {
      bulk: {
        in_progress: '开始排查',
        resolved: '标为已解决',
        closed: '批量关闭'
      },
      clearReporter: '清除提交人筛选：{email}',
      copyRef: '复制编号',
      copiedRef: '已复制 #{id}',
      copyEnv: '复制',
      copiedEnv: '已复制客户端环境',
      copySha: '复制 SHA-256',
      copiedSha: '已复制 SHA-256',
      download: '下载',
      downloadDiagnostics: '下载诊断包',
      inspectManifest: '文件清单',
      loadingManifest: '读取清单中',
      manifestEmpty: '诊断包内无文件',
      addNote: '添加备注',
      noteAndResolve: '备注并解决',
      noteAndClose: '备注并关闭',
      prev: '上一条',
      next: '下一条',
      save: '保存',
      addLabel: '新增标签'
    },
    detail: {
      noDescription: '无描述',
      internalNote: '内部备注',
      notePlaceholder: '填写内部备注',
      fixVersionPlaceholder: '修复版本',
      statusLabel: '状态',
      severityLabel: '严重程度',
      assigneeLabel: '负责人',
      assigneePlaceholder: '负责人姓名',
      labelsLabel: '标签',
      customLabelPlaceholder: '自定义标签',
      fixVersionLabel: '修复版本',
      envLabel: '客户端环境',
      envVersion: '版本',
      envOs: '系统',
      envArch: '架构',
      envRoute: '路由',
      envLocale: '语言',
      envDiagnostics: '诊断包',
      notAttached: '未附带',
      reporterLabel: '提交账号',
      viewReporterIssues: '查看提交（{count}）',
      loadFailed: '反馈详情加载失败',
      updateFailed: '更新失败',
      mediaLoadFailed: '预览加载失败',
      manifestLoadFailed: '文件清单读取失败',
      downloadFailed: '下载失败'
    },
    timeline: {
      created: '通过桌面端提交',
      createdWithDiag: '提交反馈与诊断包',
      statusChanged: '将状态从 {from} 变更为 {to}',
      statusChangedBatch: '批量将状态从 {from} 变更为 {to}',
      severityChanged: '将级别从 {from} 调整为 {to}',
      assigneeChanged: '将负责人指派给 {to}',
      assigneeCleared: '取消了负责人指派',
      fixVersionChanged: '关联修复版本 {to}',
      fixVersionCleared: '清空了修复版本',
      labelsUpdated: '更新了标签'
    }
  }
}
