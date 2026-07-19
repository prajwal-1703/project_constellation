fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(false)
        .compile(
            &[
                "../proto/agent.proto",
                "../proto/cluster.proto",
                "../proto/common.proto",
                "../proto/scheduler.proto",
            ],
            &["../proto"],
        )?;
    Ok(())
}
