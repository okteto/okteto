import { Component } from 'react';

class OAuth extends Component {
  state = {
    user: {},
    disabled: ''
  }

  componentDidMount() {
    const { socket, provider } = this.props;

    socket.on(provider, user => {  
      this.popup.close();
      this.setState({ user });
    });
  }
}

export default OAuth;