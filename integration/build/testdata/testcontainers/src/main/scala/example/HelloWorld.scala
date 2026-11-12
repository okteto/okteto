package example

object HelloWorld {

  def greeting(name: String): String = s"Hello, $name!"

  def main(args: Array[String]): Unit =
    println(greeting(args.headOption.getOrElse("world")))
}
