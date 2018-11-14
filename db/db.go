package db
import(
	"log"
	  // 把 LedisDB import 進來
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/ledis"
)

var db *ledis.DB

func Connect() {
	// 建立一個設定檔，把資料的儲存位置設定到 ./db-data
	cfg := config.NewConfigDefault()
	cfg.DataDir = "./db_data"

	// 要求建立連線
	l, _:= ledis.Open(cfg)
	_db, err := l.Select(0)

	if err != nil {
		panic(err)
	}

	db = _db
	log.Println("Connect to db successfully")
}

func Insert(s string) {
	// fishes 是這個 list 的名字（key）
	fishes := []byte("fishes")
	// 把字串 s 加到 fishes 裡面
	db.RPush(fishes, []byte(s))
}

func SelectAll() []string {
	fishes := []byte("fishes")
	// 取得 list 的長度 -> nFish
	nFish, _:= db.LLen(fishes)

	// 從 list 裡面取得所有資料
	datas, _:= db.LRange(fishes, 0, int32(nFish))

 	// 因為取出來的每一筆資料型別都是 []byte
    // 把每筆資料都轉成 string 放到 strs 裡面
	strs := []string{}
	for _, data := range datas {
		strs = append(strs, string(data))
	}

	return strs
}
