package main

type actionView struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

var actionCatalog = []actionView{
	{Name: "initialize", Summary: "创建原生核心并指定数据目录；服务启动时已执行"},
	{Name: "sources", Summary: "列出当前编译版本可用的站源"},
	{Name: "catalog", Summary: "读取站源目录页；传 query 进行搜索，传 category 指定分类"},
	{Name: "cached", Summary: "读取本地缓存的目录页，不请求站源"},
	{Name: "categories", Summary: "读取站源内容分类"},
	{Name: "sourceStatus", Summary: "读取站源任务与诊断状态"},
	{Name: "sourceJob", Summary: "启动或继续站源更新任务，command 取 update/check/checkCatalog/retrySave"},
	{Name: "cancelSourceJob", Summary: "停止站源任务"},
	{Name: "detail", Summary: "读取剧集详情与分集列表"},
	{Name: "metadata", Summary: "读取或补齐单部剧的资料"},
	{Name: "cover", Summary: "下载并返回封面本地路径"},
	{Name: "prepareCover", Summary: "预取封面，不等待完成"},
	{Name: "resolve", Summary: "解析某集的播放地址与凭证"},
	{Name: "fallback", Summary: "切换到备用线路"},
	{Name: "release", Summary: "释放播放会话"},
	{Name: "cancelPlayback", Summary: "取消进行中的播放解析"},
	{Name: "preload", Summary: "预加载下一集"},
	{Name: "suggestions", Summary: "搜索联想"},
	{Name: "recommendations", Summary: "读取红果推荐流"},
	{Name: "cachedRecommendations", Summary: "读取缓存的推荐流"},
	{Name: "rankings", Summary: "读取指定榜单的一页"},
	{Name: "rankingBoards", Summary: "列出当前版本可用的榜单"},
	{Name: "danmaku", Summary: "读取红果弹幕时间窗"},
	{Name: "downloads", Summary: "列出下载任务"},
	{Name: "enqueueDownloads", Summary: "加入下载分集"},
	{Name: "controlDownloads", Summary: "暂停、继续、删除单个下载任务"},
	{Name: "controlDownloadBatch", Summary: "批量控制下载任务"},
	{Name: "localPlayback", Summary: "读取已下载分集的本地播放信息"},
	{Name: "storage", Summary: "读取存储占用与目录信息"},
	{Name: "downloadDirectory", Summary: "读取下载根目录"},
	{Name: "moveDownloads", Summary: "迁移下载目录"},
	{Name: "resourceSettings", Summary: "读取站源访问与资源设置"},
	{Name: "saveResourceSettings", Summary: "保存站源访问与资源设置"},
	{Name: "workLease", Summary: "后台任务租约，供下载在后台继续"},
	{Name: "updateSystemProxy", Summary: "更新系统代理设置"},
	{Name: "lan", Summary: "局域网同步与推送操作"},
}
