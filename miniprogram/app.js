App({
  onLaunch() {
    const settings = wx.getStorageSync("xinwiki_settings");
    if (!settings) {
      wx.setStorageSync("xinwiki_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: ""
      });
    }
  }
});
