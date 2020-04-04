const linkifyExpr = /(\b(https?|ftp|file):\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/ig;
const notifType = {
    notice: "notice",
    error: "error"
};
const typingDebounceInterval = 3000;

Vue.component("expand-link", {
    props: ["link"],
    data: function () {
        return {
            visible: false
        }
    },
    methods: {
        select(e) {
            e.target.select();
        }
    },
    template: `
        <div class="expand-link">
            <a href="#" v-on:click.prevent="visible = !visible">🔗</a>
            <input v-if="visible" v-on:click="select" readonly type="text" :value="link" />
        </div>
    `
});

var app = new Vue({
    el: "#app",
    delimiters: ["{(", ")}"],
    data: {
        isBusy: false,
        chatOn: false,
        sidebarOn: true,
        disposed: false,
        hasSound: true,

        // Global flash / notifcation properties.
        notifTimer: null,
        notifMessage: "",
        notifType: "",

        // New activity animation in title bar. Page title is cached on load
        // to use in the animation.
        newActivity: false,
        newActivityCounter: 0,
        pageTitle: document.title,

        typingTimer: null,
        typingPeers: new Map(),

        // Form fields.
        roomName: "",
        handle: "",
        password: "",
        message: "",

        // Chat data.
        self: {},
        messages: [],
        peers: []
    },
    created: function () {
        this.initClient();
        this.initTimers();

        if (window.hasOwnProperty("_room") && _room.auth) {
            this.toggleChat();
            Client.init(_room.id);
            Client.connect();
        }
    },
    computed: {
        Client() {
            return window.Client;
        }
    },
    methods: {
        // Handle room creation.
        handleCreateRoom() {
            fetch("/api/rooms", {
                method: "post",
                body: JSON.stringify({
                    name: this.roomName,
                    password: this.password
                }),
                headers: { "Content-Type": "application/json; charset=utf-8" }
            })
                .then(resp => resp.json())
                .then(resp => {
                    this.toggleBusy();
                    if (resp.error) {
                        this.notify(resp.error, notifType.error);
                    } else {
                        document.location.replace("/r/" + resp.data.id);
                    }
                })
                .catch(err => {
                    this.toggleBusy();
                    this.notify(err, notifType.error);
                });
        },

        // Login to a room.
        handleLogin() {
            const handle = this.handle.replace(/[^a-z0-9_\-\.@]/ig, "");

            this.notify("Logging in", notifType.notice);
            fetch("/api/rooms/" + _room.id + "/login", {
                method: "post",
                body: JSON.stringify({ handle: handle, password: this.password }),
                headers: { "Content-Type": "application/json; charset=utf-8" }
            })
                .then(resp => resp.json())
                .then(resp => {
                    this.toggleBusy();
                    if (resp.error) {
                        this.notify(resp.error, notifType.error);
                        // pwdField.focus();
                        return;
                    }

                    this.clear();
                    this.deNotify();
                    this.toggleChat();
                    Client.init(_room.id);
                    Client.connect();
                })
                .catch(err => {
                    this.toggleBusy();
                    this.notify(err, notifType.error);
                });
        },

        // Capture keypresses to send message on Enter key and to broadcast
        // "typing" statuses.
        handleChatKeyPress(e) {
            if (e.keyCode == 13 && !e.shiftKey) {
                e.preventDefault();
                this.handleSendMessage();
                return;
            }

            // If it's a non "text" key, ignore.
            if (!String.fromCharCode(e.keyCode).match(/(\w|\s)/g)) {
                return;
            }

            // Debounce and wait for N seconds before sending a typing status.
            if (this.typingTimer) {
                return;
            }

            // Send the 'typing' status.
            Client.sendMessage(Client.MsgType["typing"]);

            this.typingTimer = window.setTimeout(() => {
                this.typingTimer = null;
            }, typingDebounceInterval);
        },

        handleSendMessage() {
            Client.sendMessage(Client.MsgType["message"], this.message);
            this.message = "";
            window.clearTimeout(this.typingTimer);
            this.typingTimer = null;
        },

        handleLogout() {
            if (!confirm("Logout?")) {
                return;
            }
            fetch("/api/rooms/" + _room.id + "/login", {
                method: "delete",
                headers: { "Content-Type": "application/json; charset=utf-8" }
            })
                .then(resp => resp.json())
                .then(resp => {
                    this.toggleChat();
                    document.location.reload();
                })
                .catch(err => {
                    this.notify(err, notifType.error);
                });
        },

        handleDisposeRoom() {
            if (!confirm("Disconnect all peers and destroy this room?")) {
                return;
            }
            Client.sendMessage(Client.MsgType["room.dispose"]);
        },

        // Flash notification.
        notify(msg, typ, timeout) {
            clearTimeout(this.notifTimer);
            this.notifTimer = setTimeout(function () {
                this.notifMessage = "";
                this.notifType = "";
            }.bind(this), timeout ? timeout : 3000);

            this.notifMessage = msg;
            if (typ) {
                this.notifType = typ;
            }
        },

        beep() {
            const b = document.querySelector("#beep");
            b.pause();
            b.load();
            b.play();
        },

        deNotify() {
            clearTimeout(this.notifTimer);
            this.notifMessage = "";
            this.notifType = "";
        },

        hashColor(str) {
            for (var i = 0, hash = 0; i < str.length; hash = str.charCodeAt(i++) + ((hash << 5) - hash));
            for (var i = 0, colour = "#"; i < 3; colour += ("00" + ((hash >> i++ * 8) & 0xFF).toString(16)).slice(-2));
            return colour;
        },

        formatDate(ts) {
            var t = new Date(ts),
                h = t.getHours(),
                minutes = t.getMinutes(),
                hours = ((h + 11) % 12 + 1);
            return (hours < 10 ? "0" : "")
                + hours.toString()
                + ":"
                + (minutes < 10 ? "0" : "")
                + minutes.toString()
                + " " + (h > 12 ? "PM" : "AM");
        },

        formatMessage(text) {
            const div = document.createElement("div");
            div.appendChild(document.createTextNode(text));
            return div.innerHTML.replace(/\n+/ig, "<br />")
                .replace(linkifyExpr, "<a refl='noopener noreferrer' href='$1' target='_blank'>$1</a>");
        },

        scrollToNewester() {
            this.$nextTick().then(function () {
                this.$refs["messages"].querySelector(".message:last-child").scrollIntoView();
            }.bind(this));
        },

        // Toggle busy (form button) state.
        toggleBusy() {
            this.isRequesting = !this.isRequesting;
        },

        toggleSidebar() {
            this.sidebarOn = !this.sidebarOn;
        },

        toggleChat() {
            this.chatOn = !this.chatOn;

            this.$nextTick().then(function () {
                if (!this.chatOn && this.$refs["form-password"]) {
                    this.$refs["form-password"].focus();
                    return
                }
                if (this.$refs["form-message"]) {
                    this.$refs["form-message"].focus();
                }
            }.bind(this));
        },

        // Clear all states.
        clear() {
            this.handle = "";
            this.password = "";
            this.password = "";
            this.message = "";
            this.self = {};
            this.messages = [];
            this.peers = [];
        },

        // WebSocket client event handlers.
        onConnect() {
            Client.getPeers();
        },

        onDisconnect(typ) {
            switch (typ) {
                case Client.MsgType["disconnect"]:
                    this.notify("Disconnected. Retrying ...", notifType.notice);
                    break;

                case Client.MsgType["peer.ratelimited"]:
                    this.notify("You sent too many messages", notifType.error);
                    this.toggleChat();
                    break;

                case Client.MsgType["room.full"]:
                    this.notify("Room is full", notifType.error);
                    this.toggleChat();
                    break;

                case Client.MsgType["room.dispose"]:
                    this.notify("Room diposed", notifType.error);
                    this.toggleChat();
                    this.disposed = true;
                    break;
            }
            // window.location.reload();
        },

        onReconnecting(timeout) {
            this.notify("Disconnected. Retrying ...", notifType.notice, timeout);
        },

        onPeerSelf(data) {
            this.self = {
                ...data.data,
                avatar: this.hashColor(data.data.id)
            };
        },

        onPeerJoinLeave(data, typ) {
            const peer = data.data;
            let peers = JSON.parse(JSON.stringify(this.peers));

            // Add / remove the peer from the existing list.
            if (typ === Client.MsgType["peer.join"]) {
                peers.push(peer);
            } else {
                peers = peers.filter((e) => { return e.id !== peer.id; });
            }
            this.onPeers(peers);

            // Notice in the message area;
            peer.avatar = this.hashColor(peer.id);
            this.messages.push({
                type: typ,
                peer: peer,
                timestamp: data.timestamp
            });
            this.scrollToNewester();
        },

        onPeers(data) {
            const peers = data.sort(function (a, b) {
                if (a.handle < b.handle) {
                    return -1;
                } else if (a.handle > b.handle) {
                    return 1;
                } else {
                    return 0;
                }
            });

            peers.forEach(p => {
                p.avatar = this.hashColor(p.id);
            });

            this.peers = peers;
        },

        onTyping(data) {
            if (data.data.id === this.self.id) {
                return;
            }
            this.typingPeers.set(data.data.id, { ...data.data, time: Date.now() });
            this.$forceUpdate();
        },

        onMessage(data) {
            // If the window isn't in focus, start the "new activity" animation
            // in the title bar.
            if (!document.hasFocus()) {
                this.newActivity = true;
                this.beep();
            }

            this.typingPeers.delete(data.data.peer_id);
            this.messages.push({
                type: Client.MsgType["message"],
                timestamp: data.timestamp,
                message: data.data.message,
                peer: {
                    id: data.data.peer_id,
                    handle: data.data.peer_handle,
                    avatar: this.hashColor(data.data.peer_id)
                }
            });
            this.scrollToNewester();
        },

        // Register chat client events.
        initClient() {
            Client.on(Client.MsgType["connect"], this.onConnect);
            Client.on(Client.MsgType["disconnect"], (data) => { this.onDisconnect(Client.MsgType["disconnect"]); });
            Client.on(Client.MsgType["peer.ratelimited"], (data) => { this.onDisconnect(Client.MsgType["peer.ratelimited"]); });
            Client.on(Client.MsgType["room.dispose"], (data) => { this.onDisconnect(Client.MsgType["room.dispose"]); });
            Client.on(Client.MsgType["room.full"], (data) => { this.onDisconnect(Client.MsgType["room.full"]); });
            Client.on(Client.MsgType["reconnecting"], this.onReconnecting);

            Client.on(Client.MsgType["peer.info"], this.onPeerSelf);
            Client.on(Client.MsgType["peer.list"], (data) => { this.onPeers(data.data); });
            Client.on(Client.MsgType["peer.join"], (data) => { this.onPeerJoinLeave(data, Client.MsgType["peer.join"]); });
            Client.on(Client.MsgType["peer.leave"], (data) => { this.onPeerJoinLeave(data, Client.MsgType["peer.leave"]); });
            Client.on(Client.MsgType["message"], this.onMessage);
            Client.on(Client.MsgType["typing"], this.onTyping);
        },

        initTimers() {
            // Title bar "new activity" animation.
            window.setInterval(() => {
                if (!this.newActivity) {
                    return;
                }
                if (this.newActivityCounter % 2 === 0) {
                    document.title = "[•] " + this.pageTitle;
                } else {
                    document.title = this.pageTitle;
                }
                this.newActivityCounter++;
            }, 2500);
            window.onfocus = () => {
                this.newActivity = false;
                document.title = this.pageTitle;
            };

            // Sweep "typing" statuses at regular intervals.
            window.setInterval(() => {
                let changed = false;
                this.typingPeers.forEach((p) => {
                    if ((p.time + typingDebounceInterval) < Date.now()) {
                        this.typingPeers.delete(p.id);
                        changed = true;
                    }
                });
                if (changed) {
                    this.$forceUpdate();
                }
            }, typingDebounceInterval);
        }
    }
});
