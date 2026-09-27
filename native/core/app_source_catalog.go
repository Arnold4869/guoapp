package core

type nativeSourceView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

var nativeSourceCatalog = []nativeSourceView{
	{ID: sourceHongguo, Name: "红果", Remark: "短剧 · 漫剧 · AI 剧"},
	{ID: sourceHuangdou, Name: "黄豆", Remark: "精选短剧"},
	{ID: sourceHuangju, Name: "剧果", Remark: "热门 · 最新 · 分类短剧"},
	{ID: sourceYeguo, Name: "野果", Remark: "分类短剧 · 在线搜索"},
	{ID: sourceDSD, Name: "帝果", Remark: "分类视频 · 在线搜索"},
	{ID: sourceHuangguoVideo, Name: "黄果视频", Remark: "视频剧集"},
	{ID: sourceHuangguoAI, Name: "黄果 AI", Remark: "AI 短剧"},
	{ID: sourceCloudFront, Name: "黄果旧版", Remark: "旧 API 剧库"},
}

func nativeSourceViews() []nativeSourceView {
	views := make([]nativeSourceView, 0, len(nativeSourceCatalog))
	for _, view := range nativeSourceCatalog {
		if nativeSourceAvailable(view.ID) {
			views = append(views, view)
		}
	}
	return views
}
