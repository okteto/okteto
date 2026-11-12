package example

import org.scalatest.BeforeAndAfterAll
import org.scalatest.funsuite.AnyFunSuite
import org.testcontainers.containers.PostgreSQLContainer

import java.sql.DriverManager

/** The "hello world" of a Testcontainers-backed test.
  *
  * It needs a reachable Docker daemon at TEST time, i.e. while `mvn package` runs inside the image build.
  */
final class HelloWorldSpec extends AnyFunSuite with BeforeAndAfterAll {

  private lazy val postgres: PostgreSQLContainer[_] = new PostgreSQLContainer("postgres:15-alpine")

  override def beforeAll(): Unit = postgres.start()

  override def afterAll(): Unit = postgres.stop()

  test("greeting is rendered") {
    assert(HelloWorld.greeting("world") == "Hello, world!")
  }

  test("postgres container answers a query") {
    val conn = DriverManager.getConnection(postgres.getJdbcUrl, postgres.getUsername, postgres.getPassword)
    try {
      val rs = conn.createStatement().executeQuery("select 1")
      assert(rs.next())
      assert(rs.getInt(1) == 1)
    } finally conn.close()
  }
}
