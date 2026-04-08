<template>
  <div class="dashboard">
    <h2 class="page-title">仪表盘</h2>
    <p class="page-desc">任务调度系统概览</p>
    <el-row :gutter="16" style="margin-top: 20px">
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-label">任务总数</div>
          <div class="stat-value">--</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-label">运行中</div>
          <div class="stat-value">--</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-label">今日执行</div>
          <div class="stat-value">--</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-label">失败任务</div>
          <div class="stat-value">--</div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #1d2129;
}
.page-desc {
  font-size: 14px;
  color: #86909c;
  margin-top: 4px;
}
.stat-card {
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  padding: 20px;
}
.stat-label {
  font-size: 14px;
  color: #86909c;
}
.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1d2129;
  margin-top: 8px;
}
</style>
