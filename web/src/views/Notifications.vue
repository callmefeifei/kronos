<template>
  <div class="notifications">
    <h2 class="page-title">通知记录</h2>
    <p class="page-desc">查看任务通知历史</p>
    <div class="content-card">
      <el-empty description="暂无数据" />
    </div>
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
.content-card {
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  padding: 24px;
  margin-top: 20px;
}
</style>
