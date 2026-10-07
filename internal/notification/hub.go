package notification

import (
	"errors"
	"sync"
)

// 推送消息的外层
// Card 负责通知内容，Event 负责这次推送的身份和类型
type Event struct {
	// 后面使用消息流的位置，供断线恢复使用。
	// 与 Card.ID 是两个概念
	ID string

	// 例如 notification.created、notification.updated
	Name string

	// 已编码的 JSON 内容，例如一张 Card 的 JSON
	Data string
}

var ErrHubClosed = errors.New("notification hub closed")

type Hub struct {
	mu sync.Mutex

	//一个用户可以有多个窗口（多个链接）
	//每个链接使用独立channel
	clients map[uint](map[chan Event]struct{}) //map[用户](这个用户的全部连接集合),
	// struct{}本身无含义，只是表示这个连接登记在这里，具体内容都在每个chan event里面
	//例如，h.clients[8][chA] = struct{}{}表示把chA登记为用户8的一个连接
	//用map而不是slice，因为map的delete是O(1)

	closed bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[uint](map[chan Event]struct{})),
	}
}

//通知始终保存在数据库中，以下函数不做持久化，不做重试

// subscribe给当前用户增加一个连接
// 返回接收消息的channel、取消订阅的函数和err
// 创建用户的某个窗口的对应channel、登记它、返回channel+清理函数+error
func (h *Hub) Subscribe(userID uint) (
	<-chan Event, //从这里接收通知；虽然实际返回的是双向channel，但这里写成单向可以增加约束
	func(), //结束连接时执行它
	error,
) {
	//开多个窗口却调用同一个client的情况下，需要加锁保护共享map
	h.mu.Lock()
	defer h.mu.Unlock()

	//停止连接，那就返回错误就行
	if h.closed {
		return nil, nil, ErrHubClosed
	}

	//每条链接最多暂存32条待发送消息
	ch := make(chan Event, 32)
	if h.clients[userID] == nil {
		//如果这是用户的第一个连接，就先开一个连接集合
		h.clients[userID] = make(map[chan Event]struct{})
	}

	//登记这次连接
	h.clients[userID][ch] = struct{}{}

	//取消订阅的函数
	//要返回这个函数，因为调用方拿不到mu，也不应该知道内部结构
	unSub := func() {
		//闭包
		//但和上面的lock+unlock执行时机不同
		h.mu.Lock()
		defer h.mu.Unlock()

		//获取这个user的所有连接
		conns := h.clients[userID]

		// 这个连接可能已经被 Send 或 Close 清理，所以要先检查存不存在
		//检查发现还在的话才能关闭，避免重复 close
		//还是闭包
		if _, exists := conns[ch]; !exists {
			return
		}

		//在map里移除这次连接
		//这一步删除登记，但接收方依旧拥有channel，需要再关闭它才行
		delete(conns, ch)
		//关闭channel
		close(ch)

		//如果删完发现一个链接都没了，把整个空map删掉
		if len(conns) == 0 {
			delete(h.clients, userID) //不是整个client，是client[当前user]
			//这样才能对上line64
		}
	}

	return ch, unSub, nil //只返回本窗口的channel
}

// send将消息放入当前user的所有连接
// 用户没有在线连接时，直接结束；通知仍保存在数据库中
func (h *Hub) Send(userID uint, e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.clients[userID]

	for ch := range conns {
		//取出的是key，所以就是chan Event
		select {
		//send持有锁，如果直接ch<-e，ch满的话就会阻塞等待
		//虽然send只用调用一次，但send内部每个窗口都尝试发送一次，因为一个用户可以开多个窗口
		case ch <- e:
			// 消息已进入这条连接的队列,就不用管了

		default: //带default的select是非阻塞等待；慢队列不值得再等，直接delete+close
			// 队列已满，结束这条订阅，删连接+关channel让其他连接继续工作
			//如果往满的channel再发送会等待；往关了的channel发东西会panic
			// SSE handler 发现 channel 关闭后结束响应。
			// 当前事件没有进入这条队列。客户端要重新订阅，再补收历史或重查列表。
			// Hub 本身不保存历史，新 channel 也不会自动收到遗漏的消息。
			delete(conns, ch) //在go里面，遍历 map 时删除当前 key 是合法的
			close(ch)
		}
	}

	if len(conns) == 0 {
		delete(h.clients, userID) //同样的，对应line64
	}
}

// 在服务关闭时结束全部订阅，并拒绝新的订阅
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}

	h.closed = true

	for _, connections := range h.clients {
		for ch := range connections {
			close(ch)
		}
	}

	clear(h.clients)
}
