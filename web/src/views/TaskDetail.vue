<template>
  <div class="task-detail">
    <h2 class="page-title">任务详情</h2>
    <div class="content-card">
      <el-empty description="任务 ID: {{ $route.params.id }}" />
    </div>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #1d2129;
}
.content-card {
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  padding: 24px;
  margin-top: 20px;
}
</style>
