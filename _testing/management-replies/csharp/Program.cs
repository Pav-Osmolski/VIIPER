using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text;
using Viiper.Client;

var read = typeof(ViiperClient).GetMethod("ReadReplyAsync", BindingFlags.NonPublic|BindingFlags.Static)!;
int passed=0;
foreach (var test in new[] {
    ("{\"server\":\"é🎮\"}\n",true,false),
    ("{\"server\":\"é🎮\"}",false,false),
    ("{\"ok\":true}",true,true),
    ("{\"ok\":",true,true),
    ("\n",true,false),
    ("{\n\"server\":\"é🎮\"\n}\n",true,false)
}) {
    using var stream=new FragmentStream(Encoding.UTF8.GetBytes(test.Item1),test.Item2);
    try {
        var result=await (Task<string>)read.Invoke(null,new object[]{stream,CancellationToken.None})!;
        if(test.Item3 || result!=test.Item1.TrimEnd('\n')) throw new Exception("Invalid reply result");
        if(test.Item1.EndsWith('\n') && stream.PastReply!=0) throw new Exception("Read past LF");
    } catch(IOException) when(test.Item3) { }
    passed++;
}
// Public generated API: complete before the peer closes, preserving split UTF-8.
using var listener=new TcpListener(IPAddress.Loopback,0);
listener.Start();
using var client=new ViiperClient("127.0.0.1",((IPEndPoint)listener.LocalEndpoint).Port);
var query=client.PingAsync();
using var peer=await listener.AcceptTcpClientAsync();
var network=peer.GetStream();
var one=new byte[1];
do { await network.ReadExactlyAsync(one); } while(one[0]!=0);
foreach(byte b in Encoding.UTF8.GetBytes("{\"server\":\"é🎮\",\"version\":\"test\"}\n"))
    await network.WriteAsync(new byte[]{b});
var ping=await query.WaitAsync(TimeSpan.FromSeconds(2));
if(ping.Server!="é🎮") throw new Exception("UTF-8 changed");
Console.WriteLine($"PASS: {passed+1} generated C# reply tests (including a live TCP peer held open)");

sealed class FragmentStream(byte[] input,bool reset):Stream {
    int offset;
    public int PastReply {get;private set;}
    public override int Read(byte[] b,int start,int count) {
        if(offset==input.Length) {PastReply++; if(reset) throw new IOException("synthetic connection reset"); return 0;}
        b[start]=input[offset++]; return 1;
    }
    public override ValueTask<int> ReadAsync(Memory<byte> buffer,CancellationToken token=default) {
        token.ThrowIfCancellationRequested();
        var b=new byte[buffer.Length]; int count=Read(b,0,b.Length); b.AsSpan(0,count).CopyTo(buffer.Span); return ValueTask.FromResult(count);
    }
    public override bool CanRead=>true;
    public override bool CanWrite=>false;
    public override bool CanSeek=>false;
    public override long Length=>throw new NotSupportedException();
    public override long Position {get=>throw new NotSupportedException();set=>throw new NotSupportedException();}
    public override void Flush() {}
    public override void Write(byte[] b,int offset,int count)=>throw new NotSupportedException();
    public override long Seek(long offset,SeekOrigin origin)=>throw new NotSupportedException();
    public override void SetLength(long length)=>throw new NotSupportedException();
}
