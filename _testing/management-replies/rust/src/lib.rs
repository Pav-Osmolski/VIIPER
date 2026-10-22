#[cfg(test)]
mod tests {
    use std::io::{Read, Write};
    use std::net::{SocketAddr, TcpListener};
    use std::sync::mpsc::{self, Sender};
    use std::thread::{self, JoinHandle};
    use std::time::Duration;
    use viiper_client::{AsyncViiperClient, ViiperClient};

    const COMPLETE: &str = "{\"server\":\"é🎮\",\"version\":\"test\"}\n";
    fn peer(reply: &'static str, hold: bool) -> (SocketAddr, Sender<()>, JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let (release, wait) = mpsc::channel();
        let server = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            stream.set_read_timeout(Some(Duration::from_secs(3))).unwrap();
            let mut one = [0];
            loop { stream.read_exact(&mut one).unwrap(); if one[0] == 0 { break; } }
            for b in reply.as_bytes() { stream.write_all(&[*b]).unwrap(); }
            if hold { let _ = wait.recv_timeout(Duration::from_secs(4)); }
        });
        (address, release, server)
    }

    #[test]
    fn synchronous_generated_client_completes_before_close_and_validates_reply() {
        for (reply, hold, valid) in [
            (COMPLETE, true, true),
            ("{\n\"server\":\"é🎮\",\n\"version\":\"test\"\n}\n", true, true),
            ("{\"server\":\"é🎮\",\"version\":\"test\"}", false, true),
            ("{\"server\":", false, false),
            ("{\"status\":409,\"title\":\"Conflict\",\"detail\":\"test\"}\n", true, false),
        ] {
            let (address, release, server) = peer(reply, hold);
            let (send, receive) = mpsc::channel();
            let client = thread::spawn(move || { let _ = send.send(ViiperClient::new(address).ping()); });
            let result = receive.recv_timeout(Duration::from_secs(2));
            drop(release);
            server.join().unwrap();
            client.join().unwrap();
            let result = result.expect("client waited for close after a complete line");
            assert_eq!(result.is_ok(), valid);
            if valid { assert_eq!(result.unwrap().server, "é🎮"); }
        }
    }

    #[tokio::test(flavor = "current_thread")]
    async fn asynchronous_generated_client_completes_before_close_and_validates_reply() {
        for (reply, hold, valid) in [
            (COMPLETE, true, true),
            ("{\n\"server\":\"é🎮\",\n\"version\":\"test\"\n}\n", true, true),
            ("{\"server\":\"é🎮\",\"version\":\"test\"}", false, true),
            ("{\"server\":", false, false),
            ("{\"status\":409,\"title\":\"Conflict\",\"detail\":\"test\"}\n", true, false),
        ] {
            let (address, release, server) = peer(reply, hold);
            let result = tokio::time::timeout(Duration::from_secs(2), AsyncViiperClient::new(address).ping()).await;
            drop(release);
            server.join().unwrap();
            let result = result.expect("client waited for close after a complete line");
            assert_eq!(result.is_ok(), valid);
            if valid { assert_eq!(result.unwrap().server, "é🎮"); }
        }
    }
}
