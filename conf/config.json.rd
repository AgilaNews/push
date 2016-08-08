{
    "log": {
        "path": "/data/logs/fcm_app_server/fcm_app_server.log",
        "level": "DEBUG",
        "console": true
    },
    "redis": {
        "addr": "127.0.0.1:6379"
    },
    "app_server": {
        "sender_id": "1066815885426",
        "security_key": "AIzaSyBMK2JittPIQI489utC3QVIOE-VSa4djwk"
    },
    "http_server": {
        "addr": ":8070"
    },
    "mysql": {
        "read": {
                "host": "127.0.0.1",
                "port": 3306,
                "db": "banews",
                "user": "root",
                "password": "MhxzKhl-Happy",
                "pool": 30
                },              
        "write": {
                "host": "127.0.0.1",
                "port": 3306,   
                "db": "banews",
                "user": "root",
                "password": "MhxzKhl-Happy",
                "pool": 10
                }
    }
}
