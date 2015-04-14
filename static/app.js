// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

var Client = new function() {
	var address = null,
		ping_interval = 5, // seconds
		reconnect_interval = 6,

		// actions
		TypeMessage = 1,
		TypePeers   = 2,
		TypeNotice  = 3,
		TypeHandle  = 4;

	var ws = null,
		// event hooks
		triggers = {"connect": [],
					"message": [],
					"peers": [],
					"dispose": [],
					"disconnect": [],
					"reconnecting": []},
		ping_timer = null,
		reconnect_timer = null,
		peer = {id: null, handle: null};


	// initialize and connect the websocket
	this.init = function(room_id, handle) {
		address = root.replace(/http(s?):/, "ws$1:") + ws_route + room_id + "?handle=" + handle;
	};

	this.connect = function() {
		this.connect();
	}

	// peer identification info
	this.peer = function() {
		return peer;
	}

	// ___
	// websocket hooks
	this.connect = function() {
		ws = new WebSocket(address);
		ws.onopen = function() {
			trigger("connect");
		};

		ws.onmessage = function(e) {
			processMessage(e.data);
		};

		ws.onerror = function(e) {
			ws.close();
			ws = null;
		};

		ws.onclose = function(e) {
			if(e.code == 1000) {
				trigger("dispose", [e.reason]);
			} else if(e.code != 1005) {
				trigger("disconnect");
			}
		};
	};

	// register callbacks
	this.on = function(e, callback) {
		if(triggers.hasOwnProperty(e)) {
			triggers[e].push(callback);
		}
	};

	// fetch peers list
	this.getPeers = function(callback) {
		send({"a": TypePeers});
	};

	// send a message
	this.message = function(message) {
		send({"a": TypeMessage, "m": message});
	}

	// ___ private
	// send a message via the socket
	// automatically encodes json if possible
	function send(message, json) {
		if(!ws || ws.readyState == ws.CLOSED || ws.readyState == ws.CLOSING) return;

		try {
			if(typeof(message) == "object") {
				message = JSON.stringify(message);
			}
			ws.send(message);
		} catch(e) {};
	}

	// trigger event callbacks
	function trigger(e, args) {
		for(var n=0; n<triggers[e].length; n++) {
			triggers[e][n].apply(triggers[e][n], args ? args : []);
		}
	}

	// parse received message
	function processMessage(message) {
		// parse json in the format [IDENTIFIER, payload]
		try {
			var data = JSON.parse(message);
		} catch(e) {
			return null;
		}

		switch(data.a) {
			// incoming message
			case TypeMessage:
				trigger("message", [data.p, data.h, data.m, data.t]);
			break;

			// peer list
			case TypePeers:
				trigger("peers", [data.peers, data.change]);
			break;

			// self's handle/id
			case TypeHandle:
				peer = {id: data.id, handle: data.handle};
			break;
		}
	}

	function attemptReconnection() {
		trigger("reconnecting", [reconnect_interval]);
		reconnect_timer = setTimeout(function() {
			self.connect();
		}, reconnect_interval * 1000);
	}

	var self = this;
};

// _____________________

$(document).ready(function() {
	var _route,
		messages = [],
		site_title = document.title,
		new_activity = false;
		new_activity_timer = 0;

	var beep_sound = new Audio();
		beep_sound.src = root + "/static/beep." + ( beep_sound.canPlayType("audio/mpeg") ? "mp3" : "ogg" );

	// initialize ui
	function initUI() {
		_route = root + route;

		// create a new room
		$("#form-create").submit(function() {
			$("#button-create").prop("disabled", true);

			newRoom($("#password").val(), function(error, id) {
				if(error) {
					notify(error, "error");
				} else {
					roomCreated(id);
				}
				$("#button-create").prop("disabled", false);
			});

			return false;
		});

		// login
		$("#form-login").submit(function() {
			$("#button-login").prop("disabled", true);
			notify("Logging in");
			login(room_id, $("#password").val(), function(error, token) {
				if(error) {
					notify(error, "error");
					$("#password").focus().select();
				} else {
					// user handle
					notify("Starting");
					var handle = $("#handle").val().replace(/[^a-z0-9_\-\.@]/ig, "");
					$("#form-login").remove();

					Client.init(room_id, handle);
					Client.connect();
				}
				$("#button-login").prop("disabled", false);
			});

			return false;
		});

		// chat
		$("#form-chat").submit(function() {
			var message = $.trim($("#message").val());

			if(message) {
				$("#message").val("");
				Client.message(message);
			}

			return false;
		});

		// chat textbox behaviour
		$("#message").keydown(function(e) {
			if (e.keyCode == 13 && !e.shiftKey) {
				e.preventDefault();
				$("#form-chat").submit();
			}
		});

		// dispose button
		$("#bt-dispose").click(function() {
			if(confirm("Dispose the chatroom and disconnect everyone?")) {
				$("#chat").remove();
				notify("Disposing");

				$.post(_route + room_id + "/dispose", function() {
					document.location.href = root;
				});
			}
			return false;
		});

		// reconnect button
		$("#bt-reconnect").click(function() {
			chatReconnectPrompt(false);

			notify("Trying to reconnect");
			window.setTimeout(function() {
				Client.connect();
			}, 3000);

			return false;
		});

		// limited fields
		$("textarea.charlimited").each(function() {
			$(this).wrap("<div class='charlimited-container'>");
			$(this).after('<span class="charlimit-counter"><span class="count">0</span> / ' + $(this).prop("maxlength") + '</span>');
		}).on('keypress keydown keyup', function() {
			var rem = parseInt($(this).prop("maxlength")) - $(this).val().length;
			$(this).next(".charlimit-counter").find(".count").text(rem);
		}).keypress();

		// links that turn into textboxes
		$(".expand-to-textbox").click(function() {
			var txt = $("<input>").prop("type", "text").val($(this).data("value")).addClass("replaced-text");
			$(this).hide().after(txt);
			txt.focus().select().blur(function() {
				$(".expand-to-textbox").show();
				$(this).remove();
			});

			return false;
		});

		// room url textbox
		$("#room-url").focus(function() {
			$(this).select();
		});

		// create button
		$("#button-create").prop("disabled", false);

		// password field
		$("#password").focus();

		// new activity alert
		$(window).focus(function() {
			new_activity = false;
			new_activity_timer = 0;
			document.title = site_title;
		});
		setInterval(function(e) {
			if(!new_activity) return false;

			if(new_activity_timer % 2 == 0) {
				document.title = "New messages ...";
			} else {
				document.title = site_title;
			}

			new_activity_timer += 1;
		}, 2500);
	}

	// ________________________
	// create a new room
	function newRoom(password, cb) {
		$.post(_route + "create", {password: password}, null, "json")
		.done(function(r) {
			cb(null, r.data.id);
		})
		.fail(function(r, status, error) {
			cb(getError(r.responseText), null);
		});
	}

	// prompt the user of a room creation
	function roomCreated(id) {
		var url = _route + id;

		$("#form-create").attr("action", url).attr("method", "get").unbind("submit");
		$("#form-create .create").remove();
		$("#room-url").val(url);
		$("#room-link").attr("href", url);

		$("#form-create .created").show();
	}

	function login(room_id, password, cb) {
		$.post(_route + room_id + "/login", {password: password}, null, "json")
		.done(function(r) {
			cb(null, r.data.id);
		})
		.fail(function(r, status, error) {
			cb(getError(r.responseText), null);
		});	
	}


	// show a ui notification
	function notify(message, type, callback) {
		$("#notification")
		.attr("class", "")
		.addClass(type ? type : "notice")
		.text(message)
		.fadeIn(200, function() {
			window.nt = setTimeout(function() {
				$("#notification").fadeOut(200);
				if(callback) {
					callback();
				}
			}, 3000);
		});
	}

	function deNotify() {
		clearTimeout(window.nt);
		$("#notification").fadeOut(200);
	}

	// display the chat area
	function chatOn() {
		deNotify();
		chatReconnectPrompt(false);

		$("#form-login").remove();
		$("#notice").text("").hide();
		$("#message").val("");
		$("#chat").show();
		$("#form-chat #message").focus();

	}

	// disable the chat area
	function chatOff(notice) {
		$("#form-login").remove();
		$("#chat").hide();
		$("#notice").text(notice ? notice : "").show();
	}

	// reconnection prompt
	function chatReconnectPrompt(s) {
		if(s) {
			$("#reconnect").show();
		} else {
			$("#reconnect").hide();
		}
	}

	// render chat messages
	function renderMessages() {
		$("#messages").render(messages, {
			time: {
				text: function() {
					var t = new Date(this.time * 1000),
						h = t.getHours(),
						minutes = t.getMinutes(),
						hours = ((h + 11) % 12 + 1);

					return (hours < 10 ? "0": "")
							+ hours.toString()
							+ ":"
							+ (minutes < 10 ? "0": "")
							+ minutes.toString()
							+ " " + (h > 12 ? "PM" : "AM");
				}
			},
			handle: {
				text: function(p) {
					return this.handle;
				}
			},
			avatar: {
				style: function() {
					return "background: " + hashColor(this.peer_id);
				}
			},
			message: {
				html: function(p) {
					if(this.notice) {
						$(p.element).parent().addClass("notice");
					}

					return linkify(escapeHTML(this.message).replace(/\n+/ig, "<br />"));
				}
			}
		});

		//$("#messages").linkify();
	}

	function linkify(text) {
		var exp = /(\b(https?|ftp|file):\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/ig;
		return text.replace(exp,"<a href='$1' target='_blank'>$1</a>");
	}

	// render the sidebar peers list
	function renderPeers(peers) {
		var list = [];

		// list of peers
		for(id in peers) {
			if(peers.hasOwnProperty(id)) {
				list.push({"handle": peers[id], "id": id})
			}
		}

		list.sort(function(a, b) {
			if (a.handle < b.handle) {
				return -1;
			} else if (a.handle > b.handle) {
				return 1;
			} else {
				return 0;
			}
		});

		// render
		if(list.length < 2) {
			$("#peer-count").text("Just you");
		} else {
			$("#peer-count").text(list.length + " peers");
		}

		$("#peers").render(list, {
			handle: {
				text: function(p) {
					$(p.element).parent().attr("id", this.id);

					return this.handle;
				}
			},
			avatar: {
				style: function() {
					return "background: " + hashColor(this.id);
				}
			}
		});

		$("#peers .peer").removeClass("self");
		$("#" + Client.peer().id).addClass("self");
	}

	// record an incoming message in the message store
	function recordMessage(peer_id, handle, message, time, notice) {
		messages.push({"peer_id": peer_id, "handle": handle, "message": message, "time": time, "notice": notice});
	}

	// scroll down and focus to the latest message
	function focusLatestMessage() {
		var m = $("#chat .chat");
		m.scrollTop(m.prop("scrollHeight"));
	}

	// hash a string to hex colour
	function hashColor(str) {
		for (var i = 0, hash = 0; i < str.length; hash = str.charCodeAt(i++) + ((hash << 5) - hash));
		for (var i = 0, colour = "#"; i < 3; colour += ("00" + ((hash >> i++ * 8) & 0xFF).toString(16)).slice(-2));
		
		return colour;
	}

	// escape html
	function escapeHTML(html) {
		var text = document.createTextNode(html);
		var div = document.createElement("div");
		div.appendChild(text);

		return div.innerHTML;
	}

	// get the error message from a server response
	function getError(r) {
		try {
			var j = JSON.parse(r);
		} catch(e) {
			return "Unknown error";
		}

		return j.error;
	}

	// play a beep sound
	function beep() {
		beep_sound.pause();
		beep_sound.load();
		beep_sound.play();
	}

	// alert the user of new activity
	function alertUser() {
		// play a beep if the window is out of focus + sound is enabled
		if(!document.hasFocus() && $("#sounds").prop("checked")) {
			new_activity = true;
			beep();
		}
	}

	// ___ chat routines
	// receive message
	Client.on("message", function(peer, handle, message, time) {
		recordMessage(peer, handle, message, time);
		renderMessages();
		focusLatestMessage();
		
		alertUser();
	});

	// receive peer list
	Client.on("peers", function(peers, change) {
		// someone joined or left
		if(change) {
			var m = change.handle + " " + (change.status ? "joined" : "left") + " the room";
			recordMessage(change.peer, change.handle, m, change.time, true);
			renderMessages();
			focusLatestMessage();
		}

		renderPeers(peers);
	});

	// websocket connected
	Client.on("connect", function() {
		chatOn();
		Client.getPeers();
	});

	// room's disposed
	Client.on("dispose", function(reason) {
		chatOff("Bye");
		$("#chat").remove();

		notify(reason, "error", function() {
			document.location.href = root;
		});
	});

	// disconnected
	Client.on("disconnect", function() {
		chatReconnectPrompt(true);
	});

	$(document).ready(function() {
		initUI();
	});
});
