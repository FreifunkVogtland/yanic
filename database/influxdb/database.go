package influxdb

import (
	"log"
	"sync"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/models"

	"github.com/FreifunkBremen/yanic/database"
)

const (
	MeasurementNode            = "node"     // Measurement for per-node statistics
	MeasurementGlobal          = "global"   // Measurement for summarized global statistics
	CounterMeasurementFirmware = "firmware" // Measurement for firmware statistics
	CounterMeasurementModel    = "model"    // Measurement for model statistics
	batchMaxSize               = 500
	batchTimeout               = 5 * time.Second
)

type Connection struct {
	database.Connection
	config Config
	client client.Client
	points chan *client.Point
	wg     sync.WaitGroup
}

type Config map[string]interface{}

func (c Config) Enable() bool {
	return c["enable"].(bool)
}
func (c Config) Address() string {
	return c["address"].(string)
}
func (c Config) Database() string {
	return c["database"].(string)
}
func (c Config) Username() string {
	return c["username"].(string)
}
func (c Config) Password() string {
	return c["password"].(string)
}

func init() {
	database.RegisterAdapter("influxdb", Connect)
}
func Connect(configuration interface{}) (database.Connection, error) {
	var config Config
	config = configuration.(map[string]interface{})
	if !config.Enable() {
		return nil, nil
	}
	// Make client
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     config.Address(),
		Username: config.Username(),
		Password: config.Password(),
	})

	if err != nil {
		return nil, err
	}

	db := &Connection{
		config: config,
		client: c,
		points: make(chan *client.Point, 1000),
	}

	db.wg.Add(1)
	go db.addWorker()

	return db, nil
}

func (conn *Connection) addPoint(name string, tags models.Tags, fields models.Fields, t ...time.Time) {
	point, err := client.NewPoint(name, tags.Map(), fields, t...)
	if err != nil {
		panic(err)
	}
	conn.points <- point
}

// Close all connection and clean up
func (conn *Connection) Close() {
	close(conn.points)
	conn.wg.Wait()
	conn.client.Close()
}

// stores data points in batches into the influxdb
func (conn *Connection) addWorker() {
	bpConfig := client.BatchPointsConfig{
		Database:  conn.config.Database(),
		Precision: "m",
	}

	var bp client.BatchPoints
	var err error
	var writeNow, closed bool
	timer := time.NewTimer(batchTimeout)

	for !closed {
		// wait for new points
		select {
		case point, ok := <-conn.points:
			if ok {
				if bp == nil {
					// create new batch
					timer.Reset(batchTimeout)
					if bp, err = client.NewBatchPoints(bpConfig); err != nil {
						log.Fatal(err)
					}
				}
				bp.AddPoint(point)
			} else {
				closed = true
			}
		case <-timer.C:
			if bp == nil {
				timer.Reset(batchTimeout)
			} else {
				writeNow = true
			}
		}

		// write batch now?
		if bp != nil && (writeNow || closed || len(bp.Points()) >= batchMaxSize) {
			log.Println("saving", len(bp.Points()), "points")

			if err = conn.client.Write(bp); err != nil {
				log.Fatal(err)
			}
			writeNow = false
			bp = nil
		}
	}
	timer.Stop()
	conn.wg.Done()
}