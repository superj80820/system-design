package repository

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLineTemplateRepo(t *testing.T) {
	templateFile, err := os.ReadFile("./template.yml")
	assert.Nil(t, err)

	template := make(map[string]string)
	assert.Nil(t, yaml.Unmarshal(templateFile, &template))

	lineTemplateRepo, err := CreateLineTemplate(template)
	assert.Nil(t, err)

	wishContent, err := lineTemplateRepo.ApplyTemplate("wish_content", "https://scdn.line-apps.com/n/channel_devcenter/img/flexsnapshot/clip/clip13.jpg", "上原亞衣", "http://linecorp.com/")
	assert.Nil(t, err)
	wish, err := lineTemplateRepo.ApplyTemplate("wish", []string{wishContent})
	assert.Nil(t, err)
	assert.Equal(t, `{"type":"flex","altText":"天使已出現～","contents":{"type":"carousel","contents":[{"type":"bubble","body":{"type":"box","layout":"vertical","contents":[{"type":"box","layout":"horizontal","contents":[{"type":"box","layout":"vertical","contents":[{"type":"image","url":"https://scdn.line-apps.com/n/channel_devcenter/img/flexsnapshot/clip/clip13.jpg","aspectMode":"cover","size":"full"}],"cornerRadius":"100px","width":"130px","height":"130px"},{"type":"box","layout":"vertical","contents":[{"type":"text","contents":[{"type":"span","text":"許起來！","size":"sm"}],"size":"sm","wrap":true},{"type":"box","layout":"baseline","contents":[{"type":"text","text":"上原亞衣","size":"xl"}],"spacing":"sm","margin":"md"},{"type":"text","text":"希望有幫你找到","color":"#a9a9a9","size":"sm"}]}],"spacing":"xl","paddingAll":"20px"},{"type":"box","layout":"horizontal","contents":[{"type":"button","action":{"type":"uri","label":"點我看資料","uri":"http://linecorp.com/"},"color":"#ff334b"}]}],"paddingAll":"0px"}}]}}`, wish)

	recognitionContent, err := lineTemplateRepo.ApplyTemplate("recognition_content", "https://scdn.line-apps.com/n/channel_devcenter/img/flexsnapshot/clip/clip13.jpg", "上原亞衣", "希望有幫你找到", "55%", "http://linecorp.com/")
	assert.Nil(t, err)
	recognition, err := lineTemplateRepo.ApplyTemplate("recognition", []string{recognitionContent, recognitionContent})
	assert.Nil(t, err)
	assert.Equal(t, `{"type":"flex","altText":"辨識完成！請查看～","contents":{"type":"carousel","contents":[{"type":"bubble","body":{"type":"box","layout":"vertical","contents":[{"type":"box","layout":"horizontal","contents":[{"type":"box","layout":"vertical","contents":[{"type":"image","url":"https://scdn.line-apps.com/n/channel_devcenter/img/flexsnapshot/clip/clip13.jpg","aspectMode":"cover","size":"full"}],"cornerRadius":"100px","width":"130px","height":"130px"},{"type":"box","layout":"vertical","contents":[{"type":"text","contents":[{"type":"span","text":"我猜可能是 :","size":"sm"}],"size":"sm","wrap":true},{"type":"box","layout":"baseline","contents":[{"type":"text","text":"上原亞衣","size":"xl"}],"spacing":"sm","margin":"md"},{"type":"text","text":"希望有幫你找到","color":"#a9a9a9","size":"sm"},{"type":"text","text":"相似度 :","offsetTop":"25px"},{"type":"box","layout":"vertical","contents":[{"type":"text","text":"55%","color":"#ffffff","align":"center","size":"md","offsetTop":"2px"}],"position":"absolute","cornerRadius":"100px","offsetTop":"100px","backgroundColor":"#ff334b","offsetStart":"60px","height":"25px","width":"53px"}]}],"spacing":"xl","paddingAll":"20px"},{"type":"box","layout":"horizontal","contents":[{"type":"button","action":{"type":"uri","label":"點我看資料","uri":"http://linecorp.com/"},"color":"#ff334b"}]}],"paddingAll":"0px"}},{"type":"bubble","body":{"type":"box","layout":"vertical","contents":[{"type":"box","layout":"horizontal","contents":[{"type":"box","layout":"vertical","contents":[{"type":"image","url":"https://scdn.line-apps.com/n/channel_devcenter/img/flexsnapshot/clip/clip13.jpg","aspectMode":"cover","size":"full"}],"cornerRadius":"100px","width":"130px","height":"130px"},{"type":"box","layout":"vertical","contents":[{"type":"text","contents":[{"type":"span","text":"我猜可能是 :","size":"sm"}],"size":"sm","wrap":true},{"type":"box","layout":"baseline","contents":[{"type":"text","text":"上原亞衣","size":"xl"}],"spacing":"sm","margin":"md"},{"type":"text","text":"希望有幫你找到","color":"#a9a9a9","size":"sm"},{"type":"text","text":"相似度 :","offsetTop":"25px"},{"type":"box","layout":"vertical","contents":[{"type":"text","text":"55%","color":"#ffffff","align":"center","size":"md","offsetTop":"2px"}],"position":"absolute","cornerRadius":"100px","offsetTop":"100px","backgroundColor":"#ff334b","offsetStart":"60px","height":"25px","width":"53px"}]}],"spacing":"xl","paddingAll":"20px"},{"type":"box","layout":"horizontal","contents":[{"type":"button","action":{"type":"uri","label":"點我看資料","uri":"http://linecorp.com/"},"color":"#ff334b"}]}],"paddingAll":"0px"}}]}}`, recognition)
}
