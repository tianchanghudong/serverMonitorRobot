package mongodb

import (
	"container/heap"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"servermonitorrobot/log"
	"sync"
	"time"
)

// session
type Session struct {
	*mgo.Session
	ref   int
	index int
}

// session heap
type SessionHeap []*Session

func (h SessionHeap) Len() int {
	return len(h)
}

func (h SessionHeap) Less(i, j int) bool {
	return h[i].ref < h[j].ref
}

func (h SessionHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *SessionHeap) Push(s interface{}) {
	s.(*Session).index = len(*h)
	*h = append(*h, s.(*Session))
}

func (h *SessionHeap) Pop() interface{} {
	l := len(*h)
	s := (*h)[l-1]
	s.index = -1
	*h = (*h)[:l-1]
	return s
}

type DialContext struct {
	sync.Mutex
	sessions SessionHeap
}

// goroutine safe
func Dial(url string, sessionNum int) (*DialContext, error) {
	c, err := DialWithTimeout(url, sessionNum, 10*time.Second, 5*time.Minute)
	return c, err
}

func DialWithMode(url string, sessionNum int, mode int) (*DialContext, error) {
	return DialWithTimeoutMode(url, sessionNum, 10*time.Second, 5*time.Minute, mode)
}

// goroutine safe
func DialWithTimeout(url string, sessionNum int, dialTimeout time.Duration, timeout time.Duration) (*DialContext, error) {
	return DialWithTimeoutMode(url, sessionNum, dialTimeout, timeout, int(mgo.PrimaryPreferred))
}

// goroutine safe
func DialWithTimeoutMode(url string, sessionNum int, dialTimeout time.Duration, timeout time.Duration, mode int) (*DialContext, error) {
	if sessionNum <= 0 {
		sessionNum = 100
		log.Waring("invalid sessionNum, reset to %v", sessionNum)
	}

	s, err := mgo.DialWithTimeout(url, dialTimeout)
	if err != nil {
		return nil, err
	}
	s.SetSyncTimeout(timeout)
	s.SetSocketTimeout(timeout)
	s.SetMode(mgo.Mode(mode), true)
	s.SetPoolLimit(sessionNum)

	c := new(DialContext)

	// sessions
	c.sessions = make(SessionHeap, sessionNum)
	c.sessions[0] = &Session{s, 0, 0}
	for i := 1; i < sessionNum; i++ {
		c.sessions[i] = &Session{s.New(), 0, i}
	}
	heap.Init(&c.sessions)

	return c, nil
}

// goroutine safe
func (c *DialContext) Close() {
	c.Lock()
	for _, s := range c.sessions {
		s.Close()
		if s.ref != 0 {
			log.Error("session ref = %v", s.ref)
		}
	}
	c.Unlock()
}

// goroutine safe
func (c *DialContext) Ref() *Session {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()
	c.Lock()
	s := c.sessions[0]
	if s.ref == 0 {
		s.Refresh()
	}
	s.ref++
	heap.Fix(&c.sessions, 0)
	c.Unlock()

	return s
}

// goroutine safe
func (c *DialContext) UnRef(s *Session) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()
	c.Lock()
	s.ref--
	heap.Fix(&c.sessions, s.index)
	c.Unlock()
}

////////////////////////////////////////////////////////////////////////////////
//??????????????????????????????????????????
func (c *DialContext) ExistData(db string, collection string, key string,
	val interface{}) int {

	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	count, err := s.DB(db).C(collection).Find(bson.M{key: val}).Count()
	if err != nil {
		return 0
	}
	return count
}

//??????????????????
func (c *DialContext) GetData(db string, collection string, key string,
	val interface{}, i interface{}) error {

	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	err := s.DB(db).C(collection).Find(bson.M{key: val}).One(i)

	return err
}

//goroutine safe
func (c *DialContext) GetDataAll(db string, collection string, key string, val interface{}, i interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(bson.M{key: val}).All(i)
}

//????????????????????????????????????????????????
func (c *DialContext) GetLimitDataAndSort(db string, collection string, limit int, searchValue interface{}, i interface{}, fields ...string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(searchValue).Sort(fields...).Limit(limit).All(i)
}

//?????????????????????????????????????????????
func (c *DialContext) GetDataByKeyAndTime(db string, collection string, key string, val interface{},
	startTimeKey string, endTimeKey string, targetTime time.Time, i interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(bson.M{key: val, startTimeKey: bson.M{"$lt": targetTime},
		endTimeKey: bson.M{"$gt": targetTime}}).All(i)
}

//????????????????????????
func (c *DialContext) GetTableDataAll(db string, collection string, i interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(nil).All(i)
}

//????????????????????????, ???????????????
func (c *DialContext) SelectTableDataAll(db string, collection string, dataSelector interface{}, i interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(nil).Select(dataSelector).All(i)
}

//??????????????????
//??????1?????????
//??????2???????????????
//??????3???Key???
//??????4???Value???
//??????5??????????????????
//??????6?????????????????????,??? bson.M{"Date": 1, "DeviceId": 1}
func (c *DialContext) GetDataBySelect(db string, collection string, key string,
	val interface{}, i interface{}, selectValue bson.M) error {

	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	err := s.DB(db).C(collection).Find(bson.M{key: val}).Select(selectValue).One(i)

	return err
}

//??????????????????
//??????1?????????
//??????2???????????????
//??????3???Key???
//??????4???Value???
//??????5??????????????????
//??????6?????????????????????,??? bson.M{"Date": 1, "DeviceId": 1}
func (c *DialContext) GetDataAllBySelect(db string, collection string,
	query interface{}, i interface{}, selectValue bson.M) error {

	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	err := s.DB(db).C(collection).Find(query).Select(selectValue).All(i)

	return err
}

//????????????????????????
func (c *DialContext) GetTableCount(db string, collection string) int {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	count, err := s.DB(db).C(collection).Count()
	if err != nil {
		return 0
	}
	return count
}

//????????????
func (c *DialContext) RemoveOneData(db string, collection string, key string, val interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Remove(bson.M{key: val})
}

//??????????????????
func (c *DialContext) RemoveOneDataAsync(db string, collection string, key string, val interface{},
	fun func(param interface{}), param interface{}) {

	//????????????
	go func() {

		defer func() {
			if r := recover(); r != nil {
				log.Error(r)
			}
		}()

		s := c.Ref()
		defer c.UnRef(s)

		s.DB(db).C(collection).Remove(bson.M{key: val})
		//????????????
		if fun != nil {
			fun(param)
		}
	}()

}

//????????????
func (c *DialContext) RemoveData(db string, collection string, key string, val interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Remove(bson.M{key: val})
}

//??????????????????
func (c *DialContext) RemoveMultipleData(db string, collection string, key string, val interface{}) (removeCount int, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	info, err := s.DB(db).C(collection).RemoveAll(bson.M{key: val})
	return info.Removed, err
}

//??????????????????
func (c *DialContext) RemoveAllData(db string, collection string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	s.DB(db).C(collection).RemoveAll(bson.M{})
}

//????????????
func (c *DialContext) RemoveDataByID(db string, collection string, id string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).RemoveId(id)
}

func (c *DialContext) DropCollection(db string, collection string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	s.DB(db).C(collection).DropCollection()
}

//goroutine safe
func (c *DialContext) SaveData(db string, collection string, key string, val interface{}, i interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	_, err := s.DB(db).C(collection).Upsert(bson.M{key: val}, i)
	if err != nil {
		log.Error(err)
	}
	return err
}

//goroutine safe
func (c *DialContext) SaveDataCustom(db string, collection string, selector interface{}, update interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	_, err := s.DB(db).C(collection).Upsert(selector, update)
	if err != nil {
		log.Error(err)
	}
	return err
}

// ????????????
func (c *DialContext) UpdateFields(db string, collection string, key string, val interface{}, update bson.M) (int, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	fields := bson.M{"$set": update}
	info, err := s.DB(db).C(collection).Upsert(bson.M{key: val}, fields)
	if err != nil {
		log.Error(err)
	}
	return info.Updated, err
}

//goroutine safe
func (c *DialContext) InsertData(db string, collection string, i ...interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Insert(i...)
}

//??????????????????
func (c *DialContext) InsertDataASync(db string, collection string,
	fun func(param interface{}), param interface{}, i ...interface{}) {
	//????????????
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(r)
			}
		}()

		s := c.Ref()
		defer c.UnRef(s)
		s.DB(db).C(collection).Insert(i...)

		//????????????
		if fun != nil {
			fun(param)
		}
	}()

}

//?????????????????????
func (c *DialContext) SnatchPart_GetPlayerFromDb(db string, collection string,
	key string, val interface{}, i interface{}, level int, account string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	return s.DB(db).C(collection).Find(bson.M{key: val, "Account": bson.M{"$ne": account},
		"PlayerBase.intattr.0": bson.M{"$gte": level - 10, "$lte": level + 10}}).All(i)
}

func (c *DialContext) FriendPart_GetPlayerFromDb(db string, collection string,
	i interface{}, level int, account string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	minLevel := 0
	maxLevel := 0
	if level%10 == 0 {
		minLevel = level - 10 + 1
		maxLevel = level
	} else {
		minLevel = level - level%10 + 1
		maxLevel = minLevel + 10 - 1
	}

	return s.DB(db).C(collection).Find(bson.M{"Account": bson.M{"$ne": account},
		"PlayerBase.intattr.0": bson.M{"$gte": minLevel, "$lte": maxLevel}}).All(i)
}

//goroutine safe
func (c *DialContext) GetCount(db string, collection string) (int, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	n, err := s.DB(db).C(collection).Count()
	if err != nil {
		log.Error(err)
	}

	return n, err
}

//?????? ???????????????????????????
func (c *DialContext) Sort(db string, collection string, i interface{}, limitNum int, sortParm string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(nil).Limit(limitNum).Sort(sortParm).All(i)
	if err != nil {
		log.Error(err)
	}
}

//?????? ???????????????????????????
func (c *DialContext) SortGtZero(db string, collection string, i interface{},
	limitNum int, greatThenParm string, sortParm string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(bson.M{greatThenParm: bson.M{"$gt": 0}}).Limit(limitNum).Sort(sortParm).All(i)
	if err != nil {
		log.Error(err)
	}
}

//???????????????????????????????????????
func (c *DialContext) FindAndSort(db string, collection string,
	key string, val interface{}, i interface{},
	limitNum int, greatThenParm string, sortParm ...string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(bson.M{key: val, greatThenParm: bson.M{"$gt": 0}}).Limit(limitNum).Sort(sortParm...).All(i)
	if err != nil {
		log.Error(err)
	}
}

//??????????????????
func (c *DialContext) SearchData(db string, collection string, key string, val interface{},
	i interface{}, limitNum int, searchKey string, searchValue bson.M) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(bson.M{key: val, searchKey: searchValue}).Limit(limitNum).All(i)
	if err != nil {
		log.Error(err)
	}
}

//?????????????????? ????????????vip??????0 ???searchValue???bson.M{PlayerBase.intattr.17:bson.M{"$gt": 0}}
func (c *DialContext) Search(db string, collection string, i interface{}, searchValue bson.M) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(searchValue).All(i)
	if err != nil {
		log.Error(err)
	}
	return err
}

//????????????????????????
func (c *DialContext) MapReduce(db string, collection string, searchValue bson.M, ret interface{}, mp *mgo.MapReduce) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)
	_, err := s.DB(db).C(collection).Find(searchValue).MapReduce(mp, ret)
	return err
}

//?????????????????? ????????????vip??????0 ???searchValue???bson.M{PlayerBase.intattr.17:bson.M{"$gt": 0}}
func (c *DialContext) SearchByLimitCount(db string, collection string, i interface{}, limitNum int, searchValue bson.M) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error(r)
		}
	}()

	s := c.Ref()
	defer c.UnRef(s)

	err := s.DB(db).C(collection).Find(searchValue).Limit(limitNum).All(i)
	if err != nil {
		log.Error(err)
	}
	return err
}

// ???????????????????????????????????????????????????
func (c *DialContext) EnsureIndex(db string, collection string, key []string) error {
	s := c.Ref()
	defer c.UnRef(s)

	return s.DB(db).C(collection).EnsureIndex(mgo.Index{
		Key:    key,
		Unique: false,
		Sparse: true,
	})
}

// ???????????????mongo????????????
func (c *DialContext) LiveServers() []string {
	s := c.Ref()
	defer c.UnRef(s)
	return s.LiveServers()
}
