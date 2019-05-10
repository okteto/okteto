import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Modal from 'components/Modal';
import { deleteSpace } from 'actions/spaces';

import './DeleteSpace.scss';

class DeleteSpace extends Component {
  constructor(props) {
    super(props);

    this.state = {
      space: null
    };
  }

  @autobind
  handleConfirmClick() {
    this.props.dispatch(deleteSpace(this.props.space));
    this.close();
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
  }

  close() {
    this.dialog && this.dialog.close();
  }

  render() {
    const { space } = this.props;

    return (
      <Modal 
        className="DeleteSpace"
        ref={ref => this.dialog = ref} 
        title="Delete Space"
        width={450}>
        <div className="delete-dialog-content layout vertical">
          <p>
            Are you sure you want to delete space&nbsp;
            <strong>{space.name}</strong>?
          </p>
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              disabled={!space}
              color="red"
              solid
              onClick={this.handleConfirmClick}>
              Delete
            </Button>
            <Button 
              color="grey"
              solid
              secondary
              onClick={this.handleCancelClick}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

DeleteSpace.propTypes = {
  dispatch: PropTypes.func.isRequired,
  space: PropTypes.object.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(DeleteSpace);