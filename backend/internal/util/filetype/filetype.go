// Package filetype 通过「文件魔数」(文件头部的固定字节序列)判断文件的真实类型。
//
// 为什么不信任文件名后缀或客户端传来的 Content-Type:
//   - 两者都由客户端提供,可以随意伪造
//   - 攻击者可以把 .php / .html 改名成 .jpg 上传;若服务端按后缀存盘并当静态文件伺服,
//     可能造成 XSS 甚至 RCE
//   - 只有文件头字节是文件自身携带的,伪造成本高
package filetype

import "bytes"

// 支持的图片 MIME 类型
const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEGIF  = "image/gif"
	MIMEWebP = "image/webp"
	MIMEBMP  = "image/bmp"
)

// HeaderSize 判断文件类型所需的最小头部长度
// 取最长的规则(WebP 需要 12 字节)作为统一读取长度
const HeaderSize = 12

// magic 一条魔数规则:文件头以 bytes 开头,即判定为 mime 类型
type magic struct {
	mime  string
	bytes []byte
}

// 图片魔数表
// 每条都是各格式官方规范里定义的固定文件头
var imageMagics = []magic{
	// PNG:8 字节固定签名
	{MIMEPNG, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}},
	// JPEG:以 SOI(FF D8)开头,后跟第一个段的标记 FF
	{MIMEJPEG, []byte{0xFF, 0xD8, 0xFF}},
	// GIF:两种版本号
	{MIMEGIF, []byte("GIF87a")},
	{MIMEGIF, []byte("GIF89a")},
	// BMP:只有 2 字节 "BM",判别力较弱 —— 任何以 BM 开头的文件都会被命中,
	//      所以业务层若要严格,应配合其他校验(如文件大小、能否解码)
	{MIMEBMP, []byte("BM")},
}

// DetectImage 根据文件头字节判断图片格式
//
// header 建议至少传 HeaderSize 字节;传少了也能判断多数格式,
// 只是像 WebP 这类需要 12 字节的规则会被跳过(返回 false,而非误判)
//
// 返回匹配到的 MIME 类型;无法识别时返回 "", false
func DetectImage(header []byte) (string, bool) {
	// WebP 是复合魔数:第 0-3 字节 "RIFF"、第 8-11 字节 "WEBP",中间 4 字节是文件长度
	// 它不符合上面「从头连续匹配」的简单模型,所以单独判断
	if len(header) >= 12 &&
		bytes.Equal(header[0:4], []byte("RIFF")) &&
		bytes.Equal(header[8:12], []byte("WEBP")) {
		return MIMEWebP, true
	}

	for _, m := range imageMagics {
		if bytes.HasPrefix(header, m.bytes) {
			return m.mime, true
		}
	}
	return "", false
}

// IsImage 判断文件头是否是受支持的图片格式
// 只需要「是/否」时用它,想拿具体 MIME 用 DetectImage
func IsImage(header []byte) bool {
	_, ok := DetectImage(header)
	return ok
}

// extByMIME 各 MIME 对应的文件扩展名(带点)
var extByMIME = map[string]string{
	MIMEJPEG: ".jpg",
	MIMEPNG:  ".png",
	MIMEGIF:  ".gif",
	MIMEWebP: ".webp",
	MIMEBMP:  ".bmp",
}

// ExtForMIME 返回 MIME 对应的扩展名(带点)
//
// 存盘用的后缀必须走这里,不能用用户传的文件名 ——
// 后者客户端可控,传 evil.html 就会被存成 .html,静态伺服时造成 XSS
func ExtForMIME(mime string) (string, bool) {
	ext, ok := extByMIME[mime]
	return ext, ok
}
