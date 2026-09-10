export default {
  packages: {
    title: '资源包管理',
    description: '管理员维护资源包、审核上下架、套餐/权限绑定、下载审计和存储入口。',
    actions: {
      refresh: '刷新',
      storageConfig: '存储配置',
      create: '新增资源包',
      edit: '编辑',
      audit: '审计',
      recordDownload: '记下载',
      bindings: '绑定',
      approve: '通过',
      reject: '拒绝',
      takeDown: '下架',
      delete: '删除',
      saveBindings: '保存绑定',
      saveConfig: '保存配置'
    },
    filters: {
      searchPlaceholder: '搜索名称、版本、说明或存储 Key',
      allStatuses: '全部状态'
    },
    summary: {
      total: '总资源包',
      pendingReview: '待审核',
      approved: '已通过',
      downloads: '下载次数'
    },
    cloudKey: {
      title: '云端包库密钥',
      description: '用于上传包的 cloudEnvelope 和后续授权安装；私钥只保存在服务端。',
      configured: '已配置',
      notConfigured: '未配置',
      generate: '生成密钥',
      generating: '生成中',
      sources: {
        database: '数据库',
        config: '配置文件',
        static: '静态注入',
        none: '未配置'
      }
    },
    empty: {
      title: '暂无资源包',
      description: '先新增一个资源包，再绑定套餐或权限。'
    },
    fields: {
      name: '名称',
      version: '版本',
      description: '说明',
      status: '状态',
      sizeBytes: '大小（字节）',
      fileURL: '文件 URL',
      storageKey: '存储 Key',
      checksum: '校验和',
      planBindings: '套餐绑定',
      permissionBindings: '权限绑定',
      download: '下载',
      storage: '存储',
      reviewNote: '审核备注',
      takedownReason: '下架原因',
      driver: '驱动',
      localPath: '本地路径',
      publicBaseURL: '公开 Base URL',
      endpoint: 'Endpoint',
      region: 'Region',
      bucket: 'Bucket',
      accessKeyID: 'Access Key ID',
      secretAccessKey: 'Secret Access Key',
      prefix: 'Prefix',
      presignExpiryHours: '预签名有效期（小时）',
      reuseBackupS3: '复用备份 S3 凭证',
      forcePathStyle: 'Force path style',
      time: '时间',
      package: '资源包',
      user: '用户',
      ip: 'IP',
      userAgent: 'User Agent'
    },
    dialogs: {
      create: '新增资源包',
      edit: '编辑资源包',
      bindings: '套餐/权限绑定',
      storage: '资源包存储配置',
      audits: '下载审计'
    },
    placeholders: {
      version: '1.0.0',
      fileURL: 'https://...',
      storageKey: 'packages/name.zip',
      checksum: 'sha256:...',
      commaPlans: '逗号分隔，如 basic,pro',
      commaPermissions: '逗号分隔，如 package.download',
      commaSeparated: '逗号分隔',
      publicBaseURL: 'https://cdn.example.com',
      bucketInherited: '留空则沿用备份 Bucket',
      prefix: 'matcha-packages',
      endpoint: 'https://<account_id>.r2.cloudflarestorage.com',
      region: 'auto',
      secretConfigured: '已配置，留空保持不变'
    },
    status: {
      draft: '草稿',
      pending_review: '待审核',
      approved: '已通过',
      rejected: '已拒绝',
      taken_down: '已下架'
    },
    storageDrivers: {
      local: '本地',
      s3: 'S3'
    },
    labels: {
      downloadCount: '{count} 次',
      allPackages: '全部资源包',
      noAudits: '暂无审计记录'
    },
    prompts: {
      approveNote: '审核备注（可选）',
      rejectNote: '拒绝原因（可选）',
      takedownReason: '下架原因（可选）'
    },
    confirms: {
      delete: '确定删除资源包「{name}」？'
    },
    messages: {
      saved: '资源包已保存',
      bindingsSaved: '绑定已保存',
      approved: '资源包已通过',
      rejected: '资源包已拒绝',
      takenDown: '资源包已下架',
      deleted: '资源包已删除',
      auditRecorded: '下载审计已记录',
      storageSaved: '存储配置已保存',
      cloudKeyGenerated: '云端包库密钥已生成'
    },
    errors: {
      fallback: '操作失败',
      loadFailed: '加载资源包失败',
      storageLoadFailed: '加载存储配置失败',
      saveFailed: '保存资源包失败',
      bindingsSaveFailed: '保存绑定失败',
      reviewFailed: '审核资源包失败',
      takedownFailed: '下架资源包失败',
      deleteFailed: '删除资源包失败',
      auditRecordFailed: '记录下载审计失败',
      auditLoadFailed: '加载下载审计失败',
      storageSaveFailed: '保存存储配置失败',
      cloudKeyLoadFailed: '加载云端包库密钥状态失败',
      cloudKeyGenerateFailed: '生成云端包库密钥失败',
      PACKAGE_REGISTRY_UNAVAILABLE: '资源包服务暂不可用',
      MATCHA_PACKAGE_NOT_FOUND: '资源包不存在或已被删除',
      MATCHA_PACKAGE_VERSION_NOT_FOUND: '资源包版本不存在或已被删除',
      MATCHA_PACKAGE_ENTITLEMENT_NOT_FOUND: '当前用户没有该资源包权益',
      MATCHA_PACKAGE_EXISTS: '资源包已存在',
      MATCHA_PACKAGE_VERSION_EXISTS: '资源包版本已存在',
      MATCHA_PACKAGE_STORAGE_NOT_CONFIGURED: '资源包存储尚未配置',
      MATCHA_PACKAGE_BACKUP_S3_NOT_CONFIGURED: '备份 S3 尚未配置，无法复用备份凭证',
      MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED: '云端包库密钥尚未配置',
      MATCHA_PACKAGE_CLOUD_KEY_EXISTS: '云端包库密钥已配置，不能重复生成',
      SECRET_ENCRYPTION_KEY_NOT_CONFIGURED: '未配置固定加密密钥，不能保存服务端密钥',
      MATCHA_PACKAGE_CLOUD_ENVELOPE_REQUIRED: '请重新为云端导出资源包后上传',
      MATCHA_PACKAGE_VERSION_NOT_PUBLISHABLE: '该资源包版本当前不能发布',
      MATCHA_PACKAGE_VERSION_NOT_DOWNLOADABLE: '该资源包版本当前不可下载',
      INVALID_MATCHA_PACKAGE: '资源包参数无效',
      INVALID_MATCHA_PACKAGE_OBJECT: '资源包文件参数无效',
      INVALID_MATCHA_PACKAGE_OBJECT_KEY: '资源包存储 Key 无效',
      INVALID_MATCHA_PACKAGE_ENTITLEMENT: '资源包权益参数无效',
      INVALID_MATCHA_PACKAGE_BILLING_POLICY: '资源包计费策略参数无效',
      MATCHA_PACKAGE_FORBIDDEN: '无权操作该资源包'
    }
  }
}
