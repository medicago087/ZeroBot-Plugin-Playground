// Package score 签到 by夹子+gpt4+苜蓿紫
package score

import (
	"bytes"
	"image"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/FloatTech/ZeroBot-Plugin/kanban/banner"

	"github.com/FloatTech/AnimeAPI/bilibili"
	"github.com/FloatTech/AnimeAPI/wallet"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/imgfactory"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/golang/freetype"
	"github.com/wcharczuk/go-chart/v2"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	backgroundURL = "https://iw233.cn/api.php?sort=pc"
	referer       = "https://weibo.com/"
	signinMax     = 1
	// SCOREMAX 分数上限定为1200
	SCOREMAX = 1200
)

var (
	rankArray = [...]int{0, 10, 20, 50, 100, 200, 350, 550, 750, 1000, 1200}
	engine    = control.Register("score", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "签到",
		Help:              "- 签到\n- (获得|获取)签到背景[@xxx] | (获得|获取)签到背景\n- 查看等级排名\n注:为跨群排名\n- 查看我的钱包\n- 查看钱包排名\n注:为本群排行，若群人数太多不建议使用该功能!!!",
		PrivateDataFolder: "score",
	})
)

func init() {
	cachePath := engine.DataFolder() + "cache/"
	go func() {
		_ = os.RemoveAll(cachePath)
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
		sdb = initialize(engine.DataFolder() + "score.db")
	}()

	engine.OnFullMatch("签到").Limit(ctxext.LimitByUser).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			uid := ctx.Event.UserID
			now := time.Now()
			today := now.Format("20060102")
			// 签到图片
			drawedFile := cachePath + strconv.FormatInt(uid, 10) + today + "signin.png"
			picFile := cachePath + strconv.FormatInt(uid, 10) + today + ".png"
			// 获取签到时间
			si := sdb.GetSignInByUID(uid)
			siUpdateTimeStr := si.UpdatedAt.Format("20060102")
			switch {
			case si.Count >= signinMax && siUpdateTimeStr == today:
				// 如果签到时间是今天
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("今天你已经签到过了！"))
				if file.IsExist(drawedFile) {
					ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
				}
				return
			case siUpdateTimeStr != today:
				// 如果是跨天签到就清数据
				err := sdb.InsertOrUpdateSignInCountByUID(uid, 0)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
			}
			// 更新签到次数
			err := sdb.InsertOrUpdateSignInCountByUID(uid, si.Count+1)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 更新经验
			level := sdb.GetScoreByUID(uid).Score + 1
			if level > SCOREMAX {
				level = SCOREMAX
				ctx.SendChain(message.At(uid), message.Text("你的等级已经达到上限"))
			}
			err = sdb.InsertOrUpdateScoreByUID(uid, level)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 更新钱包
			rank := getrank(level)
			add := 1 + rand.Intn(10) + rank*5 // 等级越高获得的钱越高
			err = wallet.InsertWalletOf(uid, add)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			score := wallet.GetWalletOf(uid)
			// 绘图
			getAvatar, err := initPic(picFile, uid)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			back, err := gg.LoadImage(picFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			// 避免图片过大，最大 1280*720
			back = imgfactory.Limit(back, 1280, 720)
			imgDX := back.Bounds().Dx()
			imgDY := back.Bounds().Dy()
			canvas := gg.NewContext(imgDX, imgDY)

			// draw background
			canvas.DrawImage(back, 0, 0)

			// Create smaller Aero Style boxes
			createAeroBox := func(x, y, width, height float64) {
				aeroStyle := gg.NewContext(int(width), int(height))
				aeroStyle.DrawRoundedRectangle(0, 0, width, height, 8)
				aeroStyle.SetLineWidth(2)
				aeroStyle.SetRGBA255(255, 255, 255, 100)
				aeroStyle.StrokePreserve()
				aeroStyle.SetRGBA255(255, 255, 255, 140)
				aeroStyle.Fill()
				canvas.DrawImage(aeroStyle.Image(), int(x), int(y))
			}

			// draw aero boxes for text
			createAeroBox(20, float64(imgDY-120), 280, 100)               // left bottom
			createAeroBox(float64(imgDX-272), float64(imgDY-60), 252, 40) // right bottom

			// draw info(name, coin, etc)
			hourWord := getHourWord(now)
			canvas.SetRGB255(0, 0, 0)
			data, err := file.GetLazyData(text.BoldFontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if err = canvas.ParseFontFace(data, 24); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			nickName := ctx.CardOrNickName(uid)
			getNameLengthWidth, _ := canvas.MeasureString(nickName)
			// draw aero box
			if getNameLengthWidth > 140 {
				createAeroBox(20, 40, 140+getNameLengthWidth, 100) // left top
			} else {
				createAeroBox(20, 40, 280, 100) // left top
			}

			// draw avatar
			avatar, _, err := image.Decode(bytes.NewReader(getAvatar))
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			avatarf := imgfactory.Size(avatar, 100, 100)
			canvas.DrawImage(avatarf.Circle(0).Image(), 30, 20)

			canvas.DrawString(nickName, 140, 80)
			canvas.DrawStringAnchored(hourWord, 140, 120, 0, 0)

			if err = canvas.ParseFontFace(data, 20); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.DrawStringAnchored("ATRI币 + "+strconv.Itoa(add), 40, float64(imgDY-90), 0, 0)
			canvas.DrawStringAnchored("当前ATRI币："+strconv.Itoa(score), 40, float64(imgDY-60), 0, 0)
			canvas.DrawStringAnchored("LEVEL: "+strconv.Itoa(getrank(level)), 40, float64(imgDY-30), 0, 0)

			// Draw Info(Time, etc.)
			getTime := time.Now().Format("2006-01-02 15:04:05")
			canvas.DrawStringAnchored(getTime, float64(imgDX)-146, float64(imgDY)-40, 0.5, 0.5) // time
			var nextrankScore int
			if rank < 10 {
				nextrankScore = rankArray[rank+1]
			} else {
				nextrankScore = SCOREMAX
			}
			nextLevelStyle := strconv.Itoa(level) + "/" + strconv.Itoa(nextrankScore)
			canvas.DrawStringAnchored(nextLevelStyle, 190, float64(imgDY-30), 0, 0) // time

			// Draw Zerobot-Plugin information
			canvas.SetRGB255(255, 255, 255)
			if err = canvas.ParseFontFace(data, 20); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			canvas.DrawStringAnchored("Created By Zerobot-Plugin "+banner.Version, float64(imgDX)/2, float64(imgDY)-20, 0.5, 0.5) // zbp
			canvas.SetRGB255(0, 0, 0)
			canvas.DrawStringAnchored("Created By Zerobot-Plugin "+banner.Version, float64(imgDX)/2-3, float64(imgDY)-19, 0.5, 0.5) // zbp
			canvas.SetRGB255(255, 255, 255)

			// done.
			f, err := os.Create(drawedFile)
			if err != nil {
				data, err := imgfactory.ToBytes(canvas.Image())
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
				ctx.SendChain(message.ImageBytes(data))
				return
			}
			_, err = imgfactory.WriteTo(canvas.Image(), f)
			_ = f.Close()
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})

	engine.OnPrefixGroup([]string{"获得签到背景", "获取签到背景", "签到背景"}, zero.OnlyGroup).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			param := ctx.State["args"].(string)
			var uidStr string
			if len(ctx.Event.Message) > 1 && ctx.Event.Message[1].Type == "at" {
				uidStr = ctx.Event.Message[1].Data["qq"]
			} else if param == "" {
				uidStr = strconv.FormatInt(ctx.Event.UserID, 10)
			}
			picFile := cachePath + uidStr + time.Now().Format("20060102") + ".png"
			if file.IsNotExist(picFile) {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请先签到！"))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + picFile))
		})
	engine.OnFullMatch("查看等级排名", zero.OnlyGroup).Limit(ctxext.LimitByGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			today := time.Now().Format("20060102")
			drawedFile := cachePath + today + "scoreRank.png"
			if file.IsExist(drawedFile) {
				ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
				return
			}
			st, err := sdb.GetScoreRankByTopN(10)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if len(st) == 0 {
				ctx.SendChain(message.Text("ERROR: 目前还没有人签到过"))
				return
			}
			_, err = file.GetLazyData(text.FontFile, control.Md5File, true)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			b, err := os.ReadFile(text.FontFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			font, err := freetype.ParseFont(b)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			f, err := os.Create(drawedFile)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			var bars []chart.Value
			for _, v := range st {
				if v.Score != 0 {
					bars = append(bars, chart.Value{
						Label: ctx.CardOrNickName(v.UID),
						Value: float64(v.Score),
					})
				}
			}
			err = chart.BarChart{
				Font:  font,
				Title: "等级排名(1天只刷新1次)",
				Background: chart.Style{
					Padding: chart.Box{
						Top: 40,
					},
				},
				YAxis: chart.YAxis{
					Range: &chart.ContinuousRange{
						Min: 0,
						Max: math.Ceil(bars[0].Value/10) * 10,
					},
				},
				Height:   500,
				BarWidth: 50,
				Bars:     bars,
			}.Render(chart.PNG, f)
			_ = f.Close()
			if err != nil {
				_ = os.Remove(drawedFile)
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})
}

func getHourWord(t time.Time) string {
	h := t.Hour()
	switch {
	case 6 <= h && h < 12:
		return "早上好！"
	case 12 <= h && h < 14:
		return "中午好！"
	case 14 <= h && h < 19:
		return "下午好！"
	case 19 <= h && h < 24:
		return "晚上好！"
	case 0 <= h && h < 6:
		return "凌晨好！"
	default:
		return ""
	}
}

func getrank(count int) int {
	for k, v := range rankArray {
		if count == v {
			return k
		} else if count < v {
			return k - 1
		}
	}
	return -1
}

func initPic(picFile string, uid int64) (avatar []byte, err error) {
	if file.IsExist(picFile) {
		return nil, nil
	}
	defer process.SleepAbout1sTo2s()
	url, err := bilibili.GetRealURL(backgroundURL)
	if err != nil {
		return nil, err
	}
	data, err := web.RequestDataWith(web.NewDefaultClient(), url, "", referer, "", nil)
	if err != nil {
		return nil, err
	}
	avatar, err = web.GetData("http://q4.qlogo.cn/g?b=qq&nk=" + strconv.FormatInt(uid, 10) + "&s=640")
	if err != nil {
		return nil, err
	}
	return avatar, os.WriteFile(picFile, data, 0644)
}
