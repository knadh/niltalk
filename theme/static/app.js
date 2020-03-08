customElements.define("chat-area", class ChatArea extends HTMLElement {
	constructor() {
		super();
	}

	connectedCallback() {
		this.tplChatArea = document.querySelector("template#chat-area").content;
		this.tplPeer = document.querySelector("template#chat-peer").content;
		this.peersList = this.tplChatArea.querySelector(".peers");
		this.attachShadow({ mode: "open" });
		this.shadowRoot.appendChild(this.tplChatArea.cloneNode(true));
	}

	// Update the peers list.
	updatePeers(peers) {
		// Clear current peer list.
		this.querySelectorAll(".peer").forEach(e => {
			e.remove();
		});

		if (peers.length < 2) {
			this.querySelector(".title").innerText = "Just you";
		} else {
			this.querySelector(".title").innerText = peers.length;
		}

		// For each peer, render a peer element with the #chat-peer template.
		peers.forEach((e) => {
			const p = this.tplPeer.cloneNode(true);
			p.querySelector(".peer").setAttribute("id", "peer-" + e.id);
			p.querySelector(".handle").innerText = e.handle;
			p.querySelector(".avatar").style.background = this.hashColor(e.id);
			this.appendChild(p);
		});
	}

	hashColor(str) {
		for (var i = 0, hash = 0; i < str.length; hash = str.charCodeAt(i++) + ((hash << 5) - hash));
		for (var i = 0, colour = "#"; i < 3; colour += ("00" + ((hash >> i++ * 8) & 0xFF).toString(16)).slice(-2));
		return colour;
	}
})


document.addEventListener("DOMContentLoaded", ready, false);
function ready() {
	const $ = (q) => {
		// document.querySelector.bind(document);
		const el = document.querySelector(q);
		if (!el) {
			return document.createElement("form");
		}
		return el;
	}
	const roomURI = window.hasOwnProperty("_roomURI") ? window._roomURI : "";
	let messages = [],
		siteTitle = document.title,
		newActivity = false,
		newActivityTimer = 0,
		chatArea = document.querySelector("chat-area");

	// Message sound.
	const beepSound = new Audio();
	beepSound.src = "/theme/static/beep." + (beepSound.canPlayType("audio/mpeg") ? "mp3" : "ogg");

	initUI = () => {
		// Create room form.
		$("#form-create").onsubmit = (e) => {
			const btn = e.target.querySelector("[type=submit]");
			toggleButton(btn);
			fetch("/api/rooms", {
				method: "post",
				body: JSON.stringify({
					name: "",
					password: e.target.querySelector("input[name=password]").value
				}),
				headers: { "Content-Type": "application/json; charset=utf-8" }
			})
				.then(resp => resp.json())
				.then(resp => {
					toggleButton(btn);
					if (resp.error) {
						notify(resp.error);
					} else {
						document.location.replace("/r/" + resp.data.id);
					}
				})
				.catch(err => {
					toggleButton(btn);
					notify(err);
				});
			return false;
		};

		// Login form.
		$("#form-login").onsubmit = (e) => {
			const btn = e.target.querySelector("[type=submit]"),
				handle = e.target.querySelector("input[name=handle]").value.replace(/[^a-z0-9_\-\.@]/ig, ""),
				pwdField = e.target.querySelector("input[name=password]");

			toggleButton(btn);
			notify("Logging in");

			fetch("/api/rooms/" + _roomID + "/login", {
				method: "post",
				body: JSON.stringify({ handle: handle, password: pwdField.value }),
				headers: { "Content-Type": "application/json; charset=utf-8" }
			})
				.then(resp => resp.json())
				.then(resp => {
					toggleButton(btn);
					if (resp.error) {
						notify(resp.error);
						pwdField.value = "";
						pwdField.focus();
						return;
					}

					// document.location.reload();
					Client.init(_room.id, handle);
					Client.connect();
				})
				.catch(err => {
					toggleButton(btn);
					notify(err);
				});
			return false;
		};

		// chat
		$("#form-chat").onsubmit = () => {
			var message = $.trim($("#message").val());

			if (message) {
				$("#message").val("");
				Client.message(message);
			}

			return false;
		};

		// chat textbox behaviour
		$("#message").onkeydown = (e) => {
			if (e.keyCode == 13 && !e.shiftKey) {
				e.preventDefault();
				$("#form-chat").submit();
			}
		};

		// dispose button
		$("#bt-dispose").onclick = () => {
			if (confirm("Dispose the chatroom and disconnect everyone?")) {
				$("#chat").remove();
				notify("Disposing");

				$.post(roomURI + "/" + roomID + "/dispose", function () {
					document.location.href = root;
				});
			}
			return false;
		};

		// reconnect button
		$("#bt-reconnect").onclick = () => {
			chatReconnectPrompt(false);

			notify("Trying to reconnect");
			window.setTimeout(function () {
				Client.connect();
			}, 3000);

			return false;
		};

		// limited fields
		// $("textarea.charlimited").forEach(e => {
		// 	$(this).wrap("<div class='charlimited-container'>");
		// 	$(this).after('<span class="charlimit-counter"><span class="count">0</span> / ' + $(this).prop("maxlength") + '</span>');
		// }).on('keypress keydown keyup', function () {
		// 	var rem = parseInt($(this).prop("maxlength")) - $(this).val().length;
		// 	$(this).next(".charlimit-counter").find(".count").text(rem);
		// });

		// links that turn into textboxes
		$(".expand-to-textbox").onclick = () => {
			var txt = $("<input>").prop("type", "text").val($(this).data("value")).addClass("replaced-text");
			$(this).hide().after(txt);
			txt.focus().select().blur(function () {
				$(".expand-to-textbox").show();
				$(this).remove();
			});

			return false;
		};

		// room url textbox
		$("#room-url").onfocus = () => {
			$(this).select();
		};

		// new activity alert
		window.onfocus = () => {
			newActivity = false;
			newActivityTimer = 0;
			document.title = siteTitle;
		};
		setInterval(function (e) {
			if (!newActivity) return false;

			if (newActivityTimer % 2 == 0) {
				document.title = "New messages ...";
			} else {
				document.title = siteTitle;
			}

			newActivityTimer += 1;
		}, 2500);
	}

	// toggleButton toggles the enabled/disabled state of a button.
	function toggleButton(e) {
		if (e.getAttribute("disabled")) {
			e.removeAttribute("disabled");
		} else {
			e.setAttribute("disabled", true);
		}
	}

	// toggleDisplay the display of an element.
	function toggleDisplay(e) {
		if (e.style.display === "block") {
			e.style.display = "none";
		} else {
			e.style.display = "block";
		}
	}

	// notify shows a notification.
	function notify(message, typ, callback) {
		// If there's an existing notice, remove it.
		let e = $(".notification");
		if (e) {
			window.clearTimeout(e.timer);
			e.remove();
		}

		e = document.createElement("div");
		e.classList.add("notification");
		if (typ) {
			e.classList.add(typ);
		}
		e.innerHTML = message;
		e.timer = (e => {
			window.setInterval(() => {
				e.remove();
				if (callback) {
					callback();
				}
			}, 3000)
		})(e);
		document.body.appendChild(e);
	}

	function deNotify() {
		let e = $(".notification");
		if (!e) {
			return;
		}
		window.clearTimeout(e.timer);
		e.remove();
	}

	// Show the chat area.
	function chatOn() {
		deNotify();
		$("#form-login").remove();
		$("#message").value = "";
		toggleDisplay($("#chat"));
	}

	// Clear the chat area.
	function chatOff(notice) {
		$("#form-login").remove();
		$("#chat").hide();
		$("#notice").text(notice ? notice : "").show();
	}

	// reconnection prompt
	function chatReconnectPrompt(s) {
		if (s) {
			$("#reconnect").show();
		} else {
			$("#reconnect").hide();
		}
	}

	// render chat messages
	function renderMessages() {
		$("#messages").render(messages, {
			time: {
				text: function () {
					var t = new Date(this.timestamp * 1000),
						h = t.getHours(),
						minutes = t.getMinutes(),
						hours = ((h + 11) % 12 + 1);
					return (hours < 10 ? "0" : "")
						+ hours.toString()
						+ ":"
						+ (minutes < 10 ? "0" : "")
						+ minutes.toString()
						+ " " + (h > 12 ? "PM" : "AM");
				}
			},
			handle: {
				text: function () {
					return this.data.handle;
				}
			},
			avatar: {
				style: function () {
					return "background: " + hashColor(this.data.peer_id);
				}
			},
			message: {
				html: function (p) {
					// if (this.notice) {
					// 	$(p.element).parent().addClass("notice");
					// }
					return linkify(escapeHTML(this.data.message).replace(/\n+/ig, "<br />"));
				}
			}
		});

		//$("#messages").linkify();
	}

	function linkify(text) {
		var exp = /(\b(https?|ftp|file):\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/ig;
		return text.replace(exp, "<a href='$1' target='_blank'>$1</a>");
	}

	// record an incoming message in the message store
	function recordMessage(m) {
		messages.push(m);
	}

	// scroll down and focus to the latest message
	function focusLatestMessage() {
		var m = $("#chat .chat");
		m.scrollTop(m.prop("scrollHeight"));
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
		} catch (e) {
			return "Unknown error";
		}

		return j.error;
	}

	// play a beep sound
	function beep() {
		beepSound.pause();
		beepSound.load();
		beepSound.play();
	}

	// alert the user of new activity
	function alertUser() {
		// play a beep if the window is out of focus + sound is enabled
		if (!document.hasFocus() && $("#sounds").prop("checked")) {
			newActivity = true;
			beep();
		}
	}

	// Register the chat client.
	Client.on(Client.MsgType.Message, function (data) {
		recordMessage(data);
		renderMessages();
		focusLatestMessage();
		alertUser();
	});

	// receive peer list
	Client.on(Client.MsgType.PeerList, function (data) {
		// someone joined or left
		// if (change) {
		// 	var m = change.handle + " " + (change.status ? "joined" : "left") + " the room";
		// 	recordMessage(change.peer, change.handle, m, change.time, true);
		// 	renderMessages();
		// 	focusLatestMessage();
		// }
		const peers = data.data.sort(function (a, b) {
			if (a.handle < b.handle) {
				return -1;
			} else if (a.handle > b.handle) {
				return 1;
			} else {
				return 0;
			}
		});
		chatArea.updatePeers(peers);
	});

	// websocket connected
	Client.on(Client.MsgType.Connect, function () {
		chatOn();
		Client.getPeers();
	});

	// room's disposed
	Client.on(Client.MsgType.Dispose, function (reason) {
		chatOff("Bye");
		$("#chat").remove();

		notify(reason, "error", function () {
			document.location.href = root;
		});
	});

	// disconnected
	Client.on(Client.MsgType.Message.Disconnect, function () {
		chatReconnectPrompt(true);
	});

	initUI();
};

