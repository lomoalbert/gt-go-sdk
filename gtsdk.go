package gtsdk

import (
	"fmt"
	"net/http"
	"time"
	"crypto/md5"
	"math/rand"
	"strconv"
	"encoding/json"
	"strings"
	"net/url"
	"math"
)

const (
	FN_CHALLENGE = "geetest_challenge"
	FN_VALIDATE = "geetest_validate"
	FN_SECCODE = "geetest_seccode"

	GT_STATUS_SESSION_KEY = "gt_server_status"

	API_URL = "http://api.geetest.com"
	REGISTER_HANDLER = "/register.php"
	VALIDATE_HANDLER = "/validate.php"

	VERSION = "3.2.0"
)

type Geetest struct {
	privateKey  string
	captchaID   string
	sdkVersion  string
	responseStr string
}

//CreateGeeTest 创建GeetestLib实例
func GeetestLib(privateKey, captchaID string) *Geetest {
	return &Geetest{
		privateKey:privateKey,
		captchaID:captchaID,
		sdkVersion:VERSION,
		responseStr:"",
	}
}

//PreProcess 验证初始化预处理.
func (gt *Geetest)PreProcess(userID string) int {
	status, challenge := gt.register(userID)
	gt.responseStr = gt.makeResponseFormat(status, challenge)
	return status
}

func (gt *Geetest)register(userID string) (int, string) {
	challenge := gt.registerChallenge(userID)
	if len(challenge) != 32 {
		return 0, gt.makeFailChallenge()
	}
	return 2, gt.md5Encode(append(challenge, []byte(gt.privateKey)))
}

func (gt *Geetest)GetResponseStr() string {
	return gt.responseStr
}

func (gt *Geetest)makeFailChallenge() string {
	rand.Seed(time.Now().Unix())
	rnd1 := rand.Intn(100)
	rnd2 := rand.Intn(100)
	md5_str1 := gt.md5Encode([]byte(strconv.Itoa(rnd1)))
	md5_str2 := gt.md5Encode([]byte(strconv.Itoa(rnd2)))
	challenge := md5_str1 + md5_str2[0:2]
	return challenge
}

func (gt *Geetest)makeResponseFormat(status int, challenge string) string {
	jsonmap := make(map[string]string)
	jsonmap["success"] = status
	jsonmap["gt"] = gt.captchaID
	jsonmap["challenge"] = challenge
	jsonbyte, _ := json.Marshal(jsonmap)
	return string(jsonbyte)
}

//registerChallenge
func (gt *Geetest)registerChallenge(userID string) (respbytes []byte) {
	var registerURL string
	if len(userID) != 0 {
		registerURL = fmt.Sprintf("%s%s?gt=%s&user_id=%s", API_URL, REGISTER_HANDLER, gt.captchaID, userID)
	} else {
		registerURL = fmt.Sprintf("%s%s?gt=%s", API_URL, REGISTER_HANDLER, gt.captchaID)
	}
	client := http.Client{Timeout: 2 * time.Second }
	resp, err := client.Get(registerURL)
	if err != nil || resp.StatusCode != 200 {
		return
	}
	_, err = resp.Body.Read(respbytes)
	if err != nil {
		return
	}
	return
}

//SuccessValidate 正常模式的二次验证方式.向geetest server 请求验证结果.
func (gt *Geetest)SuccessValidate(challenge, validate, seccode, userID string) bool {
	if !gt.checkParam(challenge, validate, seccode) || gt.checkResult(challenge, validate) {
		return 0
	}
	validateURL := fmt.Sprintf("%s%s", API_URL, VALIDATE_HANDLER)
	postdata := new(url.Values)
	postdata.Add("seccode", seccode)
	postdata.Add("sdk", "golang_" + VERSION)
	if userID != "" {
		postdata.Add("user_id", userID)
	}
	backinfo := gt.postValues(validateURL, postdata)
	return backinfo == gt.md5Encode([]byte(seccode))
}

func (gt *Geetest)postValues(url string, data url.Values) string {
	var respbyte []byte
	resp, err := http.PostForm(url, data)
	if err != nil {
		return ""
	}
	_, err = resp.Body.Read(respbyte)
	if err != nil {
		return ""
	}
	return string(respbyte)
}

func (gt *Geetest)checkResult(origin, validate string) bool {
	encodeStr := gt.md5Encode([]byte(gt.privateKey + "geetest" + origin))
	return encodeStr == validate
}

//FailbackValidate failback模式的二次验证方式.在本地对轨迹进行简单的判断返回验证结果.
func (gt *Geetest)FailbackValidate(challenge, validate, seccode string) int {
	if !gt.checkParam(challenge, validate, seccode) {
		return false
	}
	validate_str := strings.Split(validate, '_')
	if len(validate_str) < 3 {
		return false
	}
	encodeAns := validate_str[0]
	encodeFbii := validate_str[1]
	encodeIgi := validate_str[2]
	decodeAns := gt.decodeResponse(challenge, encodeAns)
	decodeFbii := gt.decodeResponse(challenge, encodeFbii)
	decodeIgi := gt.decodeResponse(challenge, encodeIgi)
	validateResult := gt.validateFailImage(decodeAns, decodeFbii, decodeIgi)
	return validateResult
}

func (gt *Geetest)checkParam(params ...string) bool {
	for _, param := range params {
		if strings.TrimSpace(param) == "" {
			return false
		}
	}
	return true
}

func (gt *Geetest)validateFailImage(ans, fullBgIndex, imgGrpIndex int) int{
	thread := 3
	fullBgIndexStr := gt.md5Encode([]byte(fullBgIndex))[0:10]
	imgGrpIndexStr := gt.md5Encode([]byte(imgGrpIndex))[0:10]
	var answerDecode string
	for i := 0; i < 10; i++ {
		if i % 2 == 0 {
			answerDecode += fullBgIndexStr[i]
		} else if i % 2 == 1 {
			answerDecode += imgGrpIndexStr[i]
		}
	}
	xDecode := answerDecode[4:]
	xInt := int(strconv.ParseInt(string(xDecode), 16, 32))
	result := xInt % 200
	if result < 40 {
		result = 40
	}
	if math.Abs(float64(ans - result)) < thread {
		return 1
	}
	return 0
}

func (gt *Geetest)md5Encode(values []byte) string {
	return fmt.Sprintf("%x", md5.Sum(values))
}

func (gt *Geetest)decodeRandBase(challenge string) int {
	baseStr := challenge[32:]
	tempList := new([]int)
	for _, char := range baseStr {
		tempChar := byte(char)
		result := tempChar - 48
		if (tempChar > 57) {
			result = tempChar - 87
		}
		tempList = append(tempList, result)
	}
	return tempList[0] * 36 + tempList[1]
}

func (gt *Geetest)decodeResponse(challenge, userresponse string) (res int) {
	if len(userresponse) > 100 {
		return
	}
	digits := []int{1, 2, 5, 10, 50}
	key := make(map[byte]int)
	for _, i := range challenge {
		if exist, _ := key[i]; exist {
			continue
		}
		value := digits[len(key) % 5]
		key[i] = value
	}
	for _, i := range userresponse {
		res += key[i]
	}
	res -= gt.decodeRandBase(challenge)
	return
}












