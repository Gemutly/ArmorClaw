fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("cargo:rerun-if-changed=src/grpc/proto/sidecar.proto");

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile(&["src/grpc/proto/sidecar.proto"], &["src/grpc/proto"])?;

    println!("cargo:warning=Protobuf code generated successfully");
    Ok(())
}
