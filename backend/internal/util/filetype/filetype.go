// Package filetype 通过「文件魔数」(文件头部的固定字节序列)判断文件的真实类型。
//
// 不信任文件名后缀 / 客户端 Content-Type:两者都可伪造,攻击者能把 .php / .html 改名成
// .jpg,按后缀存盘并当静态文件伺服时可能造成 XSS 甚至 RCE;只有文件头字节是文件自身
// 携带的,伪造成本高。
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

// 支持的视频容器 MIME 类型
const (
	MIMEVideoMP4  = "video/mp4"
	MIMEVideoWebM = "video/webm"
	MIMEVideoAVI  = "video/x-msvideo"
)

// HeaderSize 判断文件类型所需的最小头部长度:取最长的规则(WebP 需要 12 字节)
const HeaderSize = 12

// magic 文件头以 bytes 开头即判定为 mime
type magic struct {
	mime  string
	bytes []byte
}

// 图片魔数表,每条都是各格式规范里定义的固定文件头
var imageMagics = []magic{
	// PNG:8 字节固定签名
	{MIMEPNG, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}},
	// JPEG:以 SOI(FF D8)开头,后跟第一个段的标记 FF
	{MIMEJPEG, []byte{0xFF, 0xD8, 0xFF}},
	// GIF:两种版本号
	{MIMEGIF, []byte("GIF87a")},
	{MIMEGIF, []byte("GIF89a")},
	// BMP:仅 2 字节 "BM",判别力弱 —— 任何 BM 开头都会命中,要严格需再配合其他校验
	{MIMEBMP, []byte("BM")},
}

// DetectImage 根据文件头字节判断图片格式。header 建议至少传 HeaderSize 字节;
// 传少了多数格式仍能判断,只是 WebP 这类需 12 字节的规则会被跳过(返回 false 而非误判)。
func DetectImage(header []byte) (string, bool) {
	// WebP 复合魔数:[0:4]="RIFF"、[8:12]="WEBP",中间 4 字节是文件长度,故单独判断
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

// IsImage 只判断「是不是图片」,要具体 MIME 用 DetectImage
func IsImage(header []byte) bool {
	_, ok := DetectImage(header)
	return ok
}

var videoMagics = []magic{
	// WebM / Matroska:EBML 头
	{MIMEVideoWebM, []byte{0x1A, 0x45, 0xDF, 0xA3}},
}

// DetectVideo 根据文件头字节判断视频容器格式。MP4 / AVI 和 WebP 一样是「分散魔数」,
// 单独判断:MP4 / MOV / M4V 的 "ftyp" 在第 [4:8] 字节(前 4 字节是 box 长度);
// AVI 的 "RIFF" 在 [0:4]、"AVI " 在 [8:12]。
//
// "ftyp" 是一族容器共用的签名(MP4 / MOV / M4A / 3GP / HEIC 都有),这里一律归为
// video/mp4 —— 目的只是保证后缀是视频而不是 .html,已足够。
func DetectVideo(header []byte) (string, bool) {
	if len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp")) {
		return MIMEVideoMP4, true
	}
	if len(header) >= 12 &&
		bytes.Equal(header[0:4], []byte("RIFF")) &&
		bytes.Equal(header[8:12], []byte("AVI ")) {
		return MIMEVideoAVI, true
	}

	for _, m := range videoMagics {
		if bytes.HasPrefix(header, m.bytes) {
			return m.mime, true
		}
	}
	return "", false
}

var extByMIME = map[string]string{
	MIMEJPEG: ".jpg",
	MIMEPNG:  ".png",
	MIMEGIF:  ".gif",
	MIMEWebP: ".webp",
	MIMEBMP:  ".bmp",

	MIMEVideoMP4:  ".mp4",
	MIMEVideoWebM: ".webm",
	MIMEVideoAVI:  ".avi",
}

// ExtForMIME 返回 MIME 对应的扩展名(带点)。存盘后缀必须走这里,不能用用户传的文件名 ——
// 后者客户端可控,传 evil.html 就会被存成 .html,静态伺服时造成 XSS。
func ExtForMIME(mime string) (string, bool) {
	ext, ok := extByMIME[mime]
	return ext, ok
}
