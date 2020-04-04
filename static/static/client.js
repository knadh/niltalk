var Client = new function () {
	const MsgType = {
		"connect": "connect",
		"disconnect": "disconnect",
		"reconnecting": "reconnecting",
		"room.dispose": "room.dispose",
		"room.full": "room.full",
		"message": "message",
		"typing": "typing",
		"peer.list": "peer.list",
		"peer.info": "peer.info",
		"peer.join": "peer.join",
		"peer.leave": "peer.leave",
		"peer.ratelimited": "peer.ratelimited",
		"notice": "notice",
		"handle": "handle"
	};
	this.MsgType = MsgType;

	var wsURL = null,
		pingInterval = 5, // seconds
		reconnectInterval = 4000;

	var ws = null,
		// event hooks
		triggers = {},
		ping_timer = null,
		reconnect_timer = null,
		peer = { id: null, handle: null };


	// Initialize and connect the websocket.
	this.init = function (roomID) {
		wsURL = document.location.protocol.replace(/http(s?):/, "ws$1:") +
			document.location.host + "/ws/" + roomID;
	};

	// Peer identification info.
	this.peer = function () {
		return peer;
	}

	// websocket hooks
	this.connect = function () {
		ws = new WebSocket(wsURL);
		ws.onopen = function () {
			trigger(MsgType["connect"]);
		};

		ws.onmessage = function (e) {
			var data = {};
			try {
				data = JSON.parse(e.data);
			} catch (e) {
				return null;
			}
			trigger(data.type, data);
		};

		ws.onerror = function (e) {
			ws.close();
			ws = null;
		};

		ws.onclose = function (e) {
			if (e.code == 1000) {
				if (e.reason && MsgType.hasOwnProperty(e.reason)) {
					trigger(e.reason);
					return
				}
				trigger(MsgType["disconnect"]);
			} else if (e.code != 1005) {
				trigger(MsgType["disconnect"]);
				attemptReconnection();
			}
		};
	};

	// register callbacks
	this.on = function (typ, callback) {
		if (!triggers.hasOwnProperty(typ)) {
			triggers[typ] = [];
		}
		triggers[typ].push(callback);
	};

	// fetch peers list
	this.getPeers = function () {
		send({ "type": MsgType["peer.list"] });
	};

	// send a message
	this.sendMessage = function (typ, data) {
		send({ "type": typ, "data": data });
	}

	// ___ private
	// send a message via the socket
	// automatically encodes json if possible
	function send(message, json) {
		if (!ws || ws.readyState == ws.CLOSED || ws.readyState == ws.CLOSING) return;

		try {
			if (typeof (message) == "object") {
				message = JSON.stringify(message);
			}
			ws.send(message);
		} catch (e) {
			console.log("error: " + e);
		};
	}

	// trigger event callbacks
	function trigger(typ, data) {
		if (!triggers.hasOwnProperty(typ)) {
			return;
		}

		for (var n = 0; n < triggers[typ].length; n++) {
			triggers[typ][n].call(triggers[typ][n], data);
		}
	}

	function attemptReconnection() {
		trigger(MsgType["reconnecting"], reconnectInterval);
		reconnect_timer = setTimeout(function () {
			reconnect_timer = null;
			self.connect();
		}, reconnectInterval);
	}

	var self = this;
};
